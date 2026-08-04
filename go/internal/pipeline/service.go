package pipeline

import (
	"bytes"
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
	"github.com/invopop/jsonschema"
)

type App interface {
	AI(context.Context, string, ...ai.Option) (*ai.Response, error)
	Harness(context.Context, string, map[string]any, any, harness.Options) (*harness.Result, error)
	Call(context.Context, string, map[string]any) (map[string]any, error)
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
	schema, err := aiSchema[T]()
	if err != nil {
		return out, fmt.Errorf("build response schema: %w", err)
	}
	opts := []ai.Option{ai.WithSystem(system), ai.WithSchema(schema)}
	if model != "" {
		opts = append(opts, ai.WithModel(modelForAPI(AIModel(model))))
	}
	resp, err := s.App.AI(ctx, user, opts...)
	if err != nil {
		return out, err
	}
	if err := resp.Into(&out); err != nil {
		return out, err
	}
	return out, nil
}

func aiSchema[T any]() (json.RawMessage, error) {
	return reflectedSchema[T](false)
}

func reflectedSchema[T any](allowAdditionalProperties bool) (json.RawMessage, error) {
	reflector := jsonschema.Reflector{DoNotReference: true, AllowAdditionalProperties: allowAdditionalProperties}
	rawSchema := reflector.Reflect(new(T))
	b, err := rawSchema.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var schema map[string]any
	if err := json.Unmarshal(b, &schema); err != nil {
		return nil, err
	}
	applyPythonSchemaContractsForType(schema, reflect.TypeOf((*T)(nil)).Elem())
	return json.Marshal(schema)
}

// SchemaFor exposes the same recursively-complete schema used for Python's
// Pydantic-backed reasoner inputs and outputs.
func SchemaFor[T any]() json.RawMessage {
	raw, err := reflectedSchema[T](true)
	if err != nil {
		panic(err)
	}
	return raw
}

func harnessInto[T any](ctx context.Context, s *Service, prompt, model, cwd, projectDir string) (T, *harness.Result, error) {
	var zero T
	if err := s.acquireHarness(ctx); err != nil {
		return zero, nil, err
	}
	defer s.releaseHarness()
	rawSchema, err := aiSchema[T]()
	if err != nil {
		return zero, nil, err
	}
	var schema map[string]any
	if err := json.Unmarshal(rawSchema, &schema); err != nil {
		return zero, nil, err
	}
	var parsed T
	result, err := s.App.Harness(ctx, prompt, schema, &parsed, harness.Options{
		Provider:     "opencode",
		Model:        OpenCodeModel(model),
		Cwd:          cwd,
		ProjectDir:   projectDir,
		MaxBudgetUSD: envFloatValue("HARNESS_MAX_BUDGET_USD", 5.0),
	})
	if err != nil {
		return zero, result, err
	}
	return parsed, result, nil
}

type fieldContract struct {
	Description string
	Default     any
	HasDefault  bool
	Minimum     *float64
	Maximum     *float64
}

type modelContract struct {
	Required []string
	Fields   map[string]fieldContract
}

func applyPythonSchemaContractsForType(schema map[string]any, modelType reflect.Type) {
	for modelType.Kind() == reflect.Pointer {
		modelType = modelType.Elem()
	}
	if modelType.Kind() != reflect.Struct {
		return
	}
	if modelType.Name() != "" {
		schema["title"] = modelType.Name()
	}
	if contract, ok := pythonSchemaContracts[modelType.Name()]; ok {
		applyModelContract(schema, contract)
	}
	properties, _ := schema["properties"].(map[string]any)
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		property, _ := properties[name].(map[string]any)
		if property == nil {
			continue
		}
		childType := field.Type
		for childType.Kind() == reflect.Pointer {
			childType = childType.Elem()
		}
		if childType.Kind() == reflect.Slice || childType.Kind() == reflect.Array {
			childType = childType.Elem()
			for childType.Kind() == reflect.Pointer {
				childType = childType.Elem()
			}
			items, _ := property["items"].(map[string]any)
			if items != nil && childType.Kind() == reflect.Struct {
				applyPythonSchemaContractsForType(items, childType)
			}
			continue
		}
		if childType.Kind() == reflect.Struct {
			applyPythonSchemaContractsForType(property, childType)
		}
	}
}

func applyModelContract(node map[string]any, contract modelContract) {
	required := make([]any, len(contract.Required))
	for i, name := range contract.Required {
		required[i] = name
	}
	if len(required) == 0 {
		delete(node, "required")
	} else {
		node["required"] = required
	}
	properties, _ := node["properties"].(map[string]any)
	for name, field := range contract.Fields {
		property, _ := properties[name].(map[string]any)
		if property == nil {
			continue
		}
		if field.Description != "" {
			property["description"] = field.Description
		}
		if field.HasDefault {
			property["default"] = field.Default
		}
		if field.Minimum != nil {
			property["minimum"] = *field.Minimum
		}
		if field.Maximum != nil {
			property["maximum"] = *field.Maximum
		}
	}
}

func bounds(description string) fieldContract {
	min, max := 0.0, 1.0
	return fieldContract{Description: description, Minimum: &min, Maximum: &max}
}

func defaulted(value any, description string) fieldContract {
	return fieldContract{Description: description, Default: value, HasDefault: true}
}

var pythonSchemaContracts = map[string]modelContract{
	"WriteRequest": {Required: []string{"folder_path"}, Fields: map[string]fieldContract{
		"folder_path":       {Description: "Folder with the user's research: data, results, drafts, or an existing paper."},
		"target_venue":      defaulted(nil, "Target journal/conference, e.g. 'NeurIPS' or 'Nature Communications'."),
		"field_hint":        defaulted(nil, "Field, e.g. 'machine learning systems'."),
		"max_rounds":        {Description: "Cap on critique/repair rounds after the first full build.", Default: 3, HasDefault: true, Minimum: floatPtr(1), Maximum: floatPtr(8)},
		"allow_web":         defaulted(true, "Allow web lookups for citations and positioning scans."),
		"dry_run":           defaulted(false, "Stop after evidence, positioning, and blueprint; write no paper."),
		"quality_threshold": {Default: 0.9, HasDefault: true, Minimum: floatPtr(0), Maximum: floatPtr(1)},
		"plateau_delta":     {Default: 0.01, HasDefault: true, Minimum: floatPtr(0), Maximum: floatPtr(0.1)},
		"model":             defaulted(nil, ""),
	}},
	"Workspace":             {Required: []string{"run_id", "root", "input_dir", "paper_dir", "sections_dir", "figures_dir", "reviews_dir", "evidence_path", "positioning_path", "blueprint_path", "todo_path", "source_folder", "input_files", "has_existing_draft", "has_data_files"}},
	"PrepareWorkspaceInput": {Required: []string{"folder_path"}, Fields: map[string]fieldContract{"model": defaulted(nil, "")}},
	"BuildEvidenceInput":    {Required: []string{"workspace"}, Fields: map[string]fieldContract{"model": defaulted(nil, "")}},
	"GenerateFramesInput": {Required: []string{"workspace"}, Fields: map[string]fieldContract{
		"target_venue": defaulted(nil, ""), "field_hint": defaulted(nil, ""), "model": defaulted(nil, ""),
	}},
	"JudgeFrameInput":  {Required: []string{"frame", "evidence_digest", "persona"}, Fields: map[string]fieldContract{"target_venue": defaulted(nil, ""), "model": defaulted(nil, "")}},
	"ScanNoveltyInput": {Required: []string{"workspace", "frame_set"}, Fields: map[string]fieldContract{"model": defaulted(nil, "")}},
	"RunPositioningInput": {Required: []string{"workspace"}, Fields: map[string]fieldContract{
		"target_venue": defaulted(nil, ""), "field_hint": defaulted(nil, ""), "allow_web": defaulted(true, ""), "model": defaulted(nil, ""),
	}},
	"DesignBlueprintInput": {Required: []string{"workspace"}, Fields: map[string]fieldContract{"target_venue": defaulted(nil, ""), "model": defaulted(nil, "")}},
	"WriteSectionInput": {Required: []string{"workspace", "section"}, Fields: map[string]fieldContract{
		"prev_section": defaulted(nil, ""), "next_section": defaulted(nil, ""), "model": defaulted(nil, ""),
	}},
	"BuildFigureInput":       {Required: []string{"workspace", "figure"}, Fields: map[string]fieldContract{"model": defaulted(nil, "")}},
	"BuildBibliographyInput": {Required: []string{"workspace", "citation_needs"}, Fields: map[string]fieldContract{"allow_web": defaulted(true, ""), "model": defaulted(nil, "")}},
	"RunBuildInput":          {Required: []string{"workspace", "blueprint"}, Fields: map[string]fieldContract{"allow_web": defaulted(true, ""), "model": defaulted(nil, "")}},
	"CompilePaperInput":      {Required: []string{"workspace"}, Fields: map[string]fieldContract{"model": defaulted(nil, "")}},
	"PersonaReviewInput":     {Required: []string{"workspace", "persona", "round_no"}, Fields: map[string]fieldContract{"model": defaulted(nil, "")}},
	"NarrativeReviewInput":   {Required: []string{"workspace", "round_no"}, Fields: map[string]fieldContract{"model": defaulted(nil, "")}},
	"PlanRepairsInput":       {Required: []string{"workspace", "critique"}, Fields: map[string]fieldContract{"model": defaulted(nil, "")}},
	"ApplyRepairsInput":      {Required: []string{"workspace", "plan"}, Fields: map[string]fieldContract{"model": defaulted(nil, "")}},
	"EvidenceSummary": {Required: []string{}, Fields: map[string]fieldContract{
		"fact_count": {Default: 0, HasDefault: true}, "gaps": {Description: "Missing experiments or data, phrased as TODO items."}, "draft_assessment": defaulted("", ""), "strongest_factual_thesis": defaulted("", ""), "confident": defaulted(false, ""),
	}},
	"StoryFrame": {Required: []string{"name", "angle", "central_thesis", "title", "mini_abstract", "contribution_order", "figure_emphasis", "why_it_could_win", "risk_of_failure", "evidence_alignment", "impact_potential"}, Fields: map[string]fieldContract{
		"angle": {Description: "What the frame makes primary: e.g. speed, memory, mechanism, benchmark, reliability."}, "title": {Description: "Concrete candidate title written in this frame."}, "mini_abstract": {Description: "Concrete ~120-word candidate abstract written in this frame."}, "evidence_alignment": bounds(""), "impact_potential": bounds(""),
	}},
	"FrameSet": {Required: []string{"frames", "generation_rationale", "confident"}},
	"FrameJudgment": {Required: []string{"frame_name", "persona", "comprehension", "excitement", "credibility", "naturalness", "concerns", "confident"}, Fields: map[string]fieldContract{
		"comprehension": bounds(""), "excitement": bounds(""), "credibility": bounds(""), "naturalness": bounds("Does the title/abstract read like a top human scientist wrote it?"),
	}},
	"NoveltyScan":         {Required: []string{}, Fields: map[string]fieldContract{"positioning_openings": {Description: "Angles nearby papers leave open."}, "confident": defaulted(false, "")}},
	"PositioningDecision": {Required: []string{"winning_frame_name", "final_title", "final_abstract", "opening_thesis", "contribution_order", "rejected_alternatives", "selection_rationale", "score", "confident"}, Fields: map[string]fieldContract{"score": bounds("")}},
	"SectionSpec": {Required: []string{"index", "slug", "heading", "beats", "establishes"}, Fields: map[string]fieldContract{
		"slug": {Description: "File slug, e.g. 'intro' -> sections/01_intro.tex."}, "beats": {Description: "Ordered narrative beats this section must land."}, "establishes": {Description: "What the reader knows/believes after this section (contract for the next)."}, "requires": defaulted("", "What the previous section must have established."), "evidence_ids": {Description: "EVIDENCE.md fact ids this section may use."}, "target_words": defaulted(400, ""),
	}},
	"FigureSpec": {Required: []string{"index", "slug", "purpose", "buildable"}, Fields: map[string]fieldContract{
		"purpose": {Description: "The single takeaway the figure must show."}, "data_sources": {Description: "Files under input/ the figure is built from."}, "buildable": {Description: "False when required data is missing; becomes a TODO brief instead."}, "caption_takeaway": defaulted("", ""),
	}},
	"Blueprint":       {Required: []string{"sections", "figures", "confident"}, Fields: map[string]fieldContract{"venue_notes": defaulted("", "")}},
	"WorkerResult":    {Required: []string{"name", "status"}, Fields: map[string]fieldContract{"status": {Description: "done | failed | skipped | todo"}, "summary": defaulted("", "")}},
	"BuildReport":     {Required: []string{"sections", "figures", "bibliography", "confident"}},
	"CompileReport":   {Required: []string{"success", "attempts"}, Fields: map[string]fieldContract{"pdf_path": defaulted("", ""), "error_excerpt": defaulted("", "")}},
	"SlopViolation":   {Required: []string{"file", "line", "rule", "excerpt"}},
	"SlopReport":      {Required: []string{"violations", "score"}, Fields: map[string]fieldContract{"score": bounds("")}},
	"LocatedIssue":    {Required: []string{"section", "issue", "fix_hint", "severity"}, Fields: map[string]fieldContract{"section": {Description: "Section slug or 'front_matter' or 'global'."}, "severity": {Description: "major | minor"}}},
	"PersonaReview":   {Required: []string{"persona", "issues", "acceptance_risk", "verdict", "confident"}, Fields: map[string]fieldContract{"acceptance_risk": bounds("")}},
	"NarrativeReview": {Required: []string{"transition_issues", "promise_alignment_issues", "arc_assessment", "score", "confident"}, Fields: map[string]fieldContract{"promise_alignment_issues": {Description: "Where the body under-delivers or over-delivers vs title/abstract."}, "score": bounds("")}},
	"FidelityAudit":   {Required: []string{"unsupported_claims", "number_mismatches", "citation_issues", "blocking", "score", "confident"}, Fields: map[string]fieldContract{"blocking": {Description: "True when claims drift beyond EVIDENCE.md and the round must not pass."}, "score": bounds("")}},
	"CritiqueBundle":  {Required: []string{"round", "persona_reviews", "narrative", "fidelity", "slop", "confident"}},
	"RepairTask":      {Required: []string{"target", "instructions", "priority"}, Fields: map[string]fieldContract{"target": {Description: "Section slug, 'front_matter', 'figures', or 'bibliography'."}, "priority": {Description: "high | medium | low"}}},
	"RepairPlan":      {Required: []string{"tasks", "confident"}, Fields: map[string]fieldContract{"notes": defaulted("", "")}},
	"RoundRecord":     {Required: []string{"round", "total_score", "persona_score", "narrative_score", "fidelity_score", "slop_score", "compile_ok", "repairs_applied", "stop", "stop_reason"}},
	"WriteResult":     {Required: []string{"status", "run_id", "workspace"}, Fields: map[string]fieldContract{"pdf_path": defaulted("", ""), "title": defaulted("", ""), "final_score": defaulted(0.0, ""), "stop_reason": defaulted("", ""), "todo_path": defaulted("", ""), "review_path": defaulted("", ""), "positioning_path": defaulted("", "")}},
}

func floatPtr(value float64) *float64 { return &value }

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
	raw, err := s.App.Call(ctx, s.NodeID+"."+name, m)
	if err != nil {
		return zero, err
	}
	out, err := afx.Decode[T](raw)
	if err != nil {
		return zero, fmt.Errorf("decode %s result: %w", name, err)
	}
	return out, nil
}

// callLocalInto is used only for scalar same-node results. The current Go SDK's
// control-plane Call decoder accepts object results only; CallLocal preserves
// execution lineage and workflow events while retaining the Python int contract.
func callLocalInto[T any](ctx context.Context, s *Service, name string, input any) (T, error) {
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
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
	return strings.TrimSuffix(out.String(), "\n")
}
