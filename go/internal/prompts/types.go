// Package prompts contains the prompt contracts used by the Go port.
//
// The text intentionally mirrors the Python implementation. Keep wording and
// whitespace stable: prompt changes are behavior changes for this agent.
package prompts

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

type Workspace struct {
	Root             string   `json:"root"`
	InputDir         string   `json:"input_dir"`
	EvidencePath     string   `json:"evidence_path"`
	InputFiles       []string `json:"input_files"`
	HasExistingDraft bool     `json:"has_existing_draft"`
}

type SectionSpec struct {
	Index       int      `json:"index"`
	Slug        string   `json:"slug"`
	Heading     string   `json:"heading"`
	Beats       []string `json:"beats"`
	Establishes string   `json:"establishes"`
	Requires    string   `json:"requires"`
	EvidenceIDs []string `json:"evidence_ids"`
	FigureSlugs []string `json:"figure_slugs"`
	TargetWords int      `json:"target_words"`
}

type FigureSpec struct {
	Index           int      `json:"index"`
	Slug            string   `json:"slug"`
	Purpose         string   `json:"purpose"`
	DataSources     []string `json:"data_sources"`
	Buildable       bool     `json:"buildable"`
	CaptionTakeaway string   `json:"caption_takeaway"`
}

type StoryFrame struct {
	Name              string   `json:"name"`
	Angle             string   `json:"angle"`
	CentralThesis     string   `json:"central_thesis"`
	Title             string   `json:"title"`
	MiniAbstract      string   `json:"mini_abstract"`
	ContributionOrder []string `json:"contribution_order"`
	FigureEmphasis    []string `json:"figure_emphasis"`
	WhyItCouldWin     string   `json:"why_it_could_win"`
	RiskOfFailure     string   `json:"risk_of_failure"`
	EvidenceAlignment float64  `json:"evidence_alignment"`
	ImpactPotential   float64  `json:"impact_potential"`
}

type FrameJudgment struct {
	FrameName     string   `json:"frame_name"`
	Persona       string   `json:"persona"`
	Comprehension float64  `json:"comprehension"`
	Excitement    float64  `json:"excitement"`
	Credibility   float64  `json:"credibility"`
	Naturalness   float64  `json:"naturalness"`
	Concerns      []string `json:"concerns"`
	Confident     bool     `json:"confident"`
}

type NoveltyScan struct {
	ClosestTitles       []string `json:"closest_titles"`
	CollisionRisks      []string `json:"collision_risks"`
	PositioningOpenings []string `json:"positioning_openings"`
	Confident           bool     `json:"confident"`
}

func fill(template string, values ...string) string {
	template = strings.ReplaceAll(template, "§", "`")
	oldnew := make([]string, 0, len(values))
	for i, value := range values {
		oldnew = append(oldnew, fmt.Sprintf("{%d}", i), value)
	}
	return strings.NewReplacer(oldnew...).Replace(template)
}

func specified(value string) string {
	if value == "" {
		return "not specified"
	}
	return value
}

// PythonRepr renders JSON-compatible Go values in the repr style produced by
// Pydantic's model_dump(), which the original meta-selection prompt embeds.
func PythonRepr(value any) string { return pythonRepr(reflect.ValueOf(value)) }

func pythonRepr(v reflect.Value) string {
	if !v.IsValid() {
		return "None"
	}
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return "None"
		}
		return pythonRepr(v.Elem())
	}
	switch v.Kind() {
	case reflect.String:
		return pythonString(v.String())
	case reflect.Bool:
		if v.Bool() {
			return "True"
		}
		return "False"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		rendered := strconv.FormatFloat(v.Float(), 'g', -1, v.Type().Bits())
		if !strings.ContainsAny(rendered, ".eE") {
			rendered += ".0"
		}
		return rendered
	case reflect.Slice, reflect.Array:
		parts := make([]string, v.Len())
		for i := range parts {
			parts[i] = pythonRepr(v.Index(i))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case reflect.Struct:
		parts := make([]string, 0, v.NumField())
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" {
				name = field.Name
			}
			if name == "-" {
				continue
			}
			parts = append(parts, pythonString(name)+": "+pythonRepr(v.Field(i)))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case reflect.Map:
		parts := make([]string, 0, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			parts = append(parts, pythonRepr(iter.Key())+": "+pythonRepr(iter.Value()))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return fmt.Sprint(v.Interface())
	}
}

func pythonString(s string) string {
	quote := "'"
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = "\""
	}
	var out strings.Builder
	out.WriteString(quote)
	for _, r := range s {
		switch r {
		case '\\':
			out.WriteString(`\\`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if string(r) == quote {
				out.WriteByte('\\')
				out.WriteRune(r)
			} else if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&out, `\x%02x`, r)
			} else if !unicode.IsPrint(r) {
				if r <= 0xffff {
					fmt.Fprintf(&out, `\u%04x`, r)
				} else {
					fmt.Fprintf(&out, `\U%08x`, r)
				}
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteString(quote)
	return out.String()
}
