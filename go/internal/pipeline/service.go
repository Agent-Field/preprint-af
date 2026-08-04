package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/Agent-Field/agentfield/sdk/go/ai"
	"github.com/Agent-Field/agentfield/sdk/go/harness"
	"github.com/Agent-Field/preprint-af/go/internal/afx"
	"github.com/Agent-Field/preprint-af/go/internal/harnessx"
	"github.com/invopop/jsonschema"
)

type App interface {
	AI(context.Context, string, ...ai.Option) (*ai.Response, error)
	Harness(context.Context, string, map[string]any, any, harness.Options) (*harness.Result, error)
	CallLocal(context.Context, string, map[string]any) (any, error)
}

type Service struct {
	App          App
	NodeID       string
	harnessSlots chan struct{}
}

func New(app App, nodeID string) *Service {
	width := envIntValue("OPENCODE_MAX_CONCURRENT", 10)
	return &Service{App: app, NodeID: nodeID, harnessSlots: make(chan struct{}, width)}
}

func modelForAPI(model string) string { return strings.TrimPrefix(model, "openrouter/") }

func aiInto[T any](ctx context.Context, s *Service, system, user, model string) (T, error) {
	var out T
	// The SDK's struct shortcut intentionally emits only a shallow schema. That
	// is insufficient for nested paper models: an array such as `sections`
	// otherwise has no item schema, so providers may legally return [] or put
	// scalar values where []string is required. Supply the complete recursive
	// schema, matching Pydantic's nested structured-output contract.
	schema, err := aiSchema[T]()
	if err != nil {
		return out, fmt.Errorf("build response schema: %w", err)
	}
	opts := []ai.Option{ai.WithSystem(system), ai.WithSchema(schema)}
	if model != "" {
		opts = append(opts, ai.WithModel(modelForAPI(AIModel(model))))
	}
	resp, err := s.App.AI(ctx, user, opts...)
	primaryErr := err
	if primaryErr == nil {
		if decodeErr := resp.Into(&out); decodeErr != nil {
			primaryErr = decodeErr
		} else if !semanticallyEmpty(reflect.ValueOf(out)) {
			return out, nil
		} else {
			primaryErr = fmt.Errorf("empty structured response")
		}
	}

	// Some OpenRouter models accept response_format but ignore the nested JSON
	// schema, returning {} (or its all-zero equivalent). Preserve the exact
	// authored prompts on the primary request, then make one bounded transport
	// repair in JSON mode. The schema suffix is generated, not authored prompt
	// content, and prevents silent propagation of an empty reasoning result.
	repairUser := user + "\n\nThe prior structured response was empty. Return ONLY a substantive json object matching this json schema; populate every field and obey all cardinalities:\n" + string(schema)
	repairOpts := []ai.Option{ai.WithSystem(system), ai.WithJSONMode()}
	if model != "" {
		repairOpts = append(repairOpts, ai.WithModel(modelForAPI(AIModel(model))))
	}
	resp, err = s.App.AI(ctx, repairUser, repairOpts...)
	if err != nil {
		return out, fmt.Errorf("structured response failed (%v); json repair failed: %w", primaryErr, err)
	}
	if err := resp.Into(&out); err != nil {
		return out, fmt.Errorf("structured response failed (%v); decode json repair: %w", primaryErr, err)
	}
	if semanticallyEmpty(reflect.ValueOf(out)) {
		return out, fmt.Errorf("model returned an empty structured response after one repair")
	}
	return out, nil
}

func semanticallyEmpty(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		return v.IsNil() || semanticallyEmpty(v.Elem())
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !semanticallyEmpty(v.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	default:
		return v.IsZero()
	}
}

func aiSchema[T any]() (json.RawMessage, error) {
	reflector := jsonschema.Reflector{DoNotReference: true}
	rawSchema := reflector.Reflect(new(T))
	b, err := rawSchema.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var schema map[string]any
	if err := json.Unmarshal(b, &schema); err != nil {
		return nil, err
	}
	// These cardinalities are already explicit in the original prompts. Encode
	// them structurally as well so models cannot satisfy strict JSON mode with
	// a vacuous empty result and bypass the intended tournament/blueprint.
	t := reflect.TypeOf((*T)(nil)).Elem()
	switch t {
	case reflect.TypeOf(FrameSet{}):
		constrainArray(schema, "frames", 5, 6)
	case reflect.TypeOf(Blueprint{}):
		constrainArray(schema, "sections", 6, 9)
	}
	return json.Marshal(schema)
}

func constrainArray(schema map[string]any, name string, min, max int) {
	properties, _ := schema["properties"].(map[string]any)
	property, _ := properties[name].(map[string]any)
	if property == nil {
		return
	}
	property["minItems"] = min
	property["maxItems"] = max
}

func harnessInto[T any](ctx context.Context, s *Service, prompt, model, cwd, projectDir string) (T, *harness.Result, error) {
	var zero T
	if err := s.acquireHarness(ctx); err != nil {
		return zero, nil, err
	}
	defer s.releaseHarness()
	parsed, result, err := harnessx.Run[T](ctx, s.App, prompt, harness.Options{
		Provider:     "opencode",
		Model:        OpenCodeModel(model),
		Cwd:          cwd,
		ProjectDir:   projectDir,
		MaxBudgetUSD: envFloatValue("HARNESS_MAX_BUDGET_USD", 5.0),
	})
	if err != nil {
		return zero, result, err
	}
	if parsed == nil {
		return zero, result, nil
	}
	return *parsed, result, nil
}

func (s *Service) acquireHarness(ctx context.Context) error {
	select {
	case s.harnessSlots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) releaseHarness() { <-s.harnessSlots }

func envIntValue(key string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return fallback
}
func envFloatValue(key string, fallback float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil && v > 0 {
		return v
	}
	return fallback
}

func callInto[T any](ctx context.Context, s *Service, name string, input any) (T, error) {
	var zero T
	m, err := afx.Map(input)
	if err != nil {
		return zero, err
	}
	raw, err := s.App.CallLocal(ctx, name, m)
	if err != nil {
		return zero, err
	}
	out, err := afx.Decode[T](raw)
	if err != nil {
		return zero, fmt.Errorf("decode %s result: %w", name, err)
	}
	return out, nil
}

func compactJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func prettyJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
