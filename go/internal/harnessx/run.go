package harnessx

import (
	"context"
	"encoding/json"

	"github.com/Agent-Field/agentfield/sdk/go/harness"
	"github.com/invopop/jsonschema"
)

type Caller interface {
	Harness(context.Context, string, map[string]any, any, harness.Options) (*harness.Result, error)
}

func Run[T any](ctx context.Context, app Caller, prompt string, opts harness.Options) (*T, *harness.Result, error) {
	reflector := jsonschema.Reflector{DoNotReference: true}
	raw := reflector.Reflect(new(T))
	var schema map[string]any
	b, err := raw.MarshalJSON()
	if err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(b, &schema); err != nil {
		return nil, nil, err
	}
	var dest T
	result, err := app.Harness(ctx, prompt, schema, &dest, opts)
	if err != nil {
		return nil, result, err
	}
	return &dest, result, nil
}
