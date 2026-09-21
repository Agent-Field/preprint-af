package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"
)

// WriteRequest is the public request accepted by the write-paper entry reasoner.
// Optional strings are pointers so their JSON representation remains null, matching
// Pydantic's model_dump/model_dump_json behavior.
type WriteRequest struct {
	FolderPath       string  `json:"folder_path"`
	TargetVenue      *string `json:"target_venue"`
	FieldHint        *string `json:"field_hint"`
	MaxRounds        int     `json:"max_rounds"`
	AllowWeb         bool    `json:"allow_web"`
	DryRun           bool    `json:"dry_run"`
	QualityThreshold float64 `json:"quality_threshold"`
	PlateauDelta     float64 `json:"plateau_delta"`
	Model            *string `json:"model"`
}

func NewWriteRequest(folderPath string) WriteRequest {
	return WriteRequest{
		FolderPath:       folderPath,
		MaxRounds:        3,
		AllowWeb:         true,
		QualityThreshold: 0.90,
		PlateauDelta:     0.01,
	}
}

// UnmarshalJSON seeds the entry reasoner's call defaults before binding. The
// Pydantic data model itself defaults max_rounds to 3, while write_paper's
// public signature defaults it to 6; JSON requests bind at that public surface.
func (r *WriteRequest) UnmarshalJSON(data []byte) error {
	type plain WriteRequest
	seeded := plain(NewWriteRequest(""))
	seeded.MaxRounds = 6
	if err := json.Unmarshal(data, &seeded); err != nil {
		return err
	}
	*r = WriteRequest(seeded)
	return nil
}

func (r WriteRequest) Validate() error {
	if r.FolderPath == "" {
		return fmt.Errorf("folder_path is required")
	}
	if r.MaxRounds < 1 || r.MaxRounds > 8 {
		return fmt.Errorf("max_rounds must be between 1 and 8")
	}
	if r.QualityThreshold < 0 || r.QualityThreshold > 1 {
		return fmt.Errorf("quality_threshold must be between 0 and 1")
	}
	if r.PlateauDelta < 0 || r.PlateauDelta > 0.1 {
		return fmt.Errorf("plateau_delta must be between 0 and 0.1")
	}
	return nil
}

type Workspace struct {
	RunID            string   `json:"run_id"`
	Root             string   `json:"root"`
	InputDir         string   `json:"input_dir"`
	PaperDir         string   `json:"paper_dir"`
	SectionsDir      string   `json:"sections_dir"`
	FiguresDir       string   `json:"figures_dir"`
	ReviewsDir       string   `json:"reviews_dir"`
	EvidencePath     string   `json:"evidence_path"`
	PositioningPath  string   `json:"positioning_path"`
	BlueprintPath    string   `json:"blueprint_path"`
	TODOPath         string   `json:"todo_path"`
	SourceFolder     string   `json:"source_folder"`
	InputFiles       []string `json:"input_files"`
	HasExistingDraft bool     `json:"has_existing_draft"`
	HasDataFiles     bool     `json:"has_data_files"`
}

type EvidenceSummary struct {
	FactCount              int      `json:"fact_count"`
	FigureCandidates       []string `json:"figure_candidates"`
	Gaps                   []string `json:"gaps"`
	ExistingCitations      []string `json:"existing_citations"`
	DraftAssessment        string   `json:"draft_assessment"`
	StrongestFactualThesis string   `json:"strongest_factual_thesis"`
	Confident              bool     `json:"confident"`
}

func NewEvidenceSummary() EvidenceSummary {
	return EvidenceSummary{FigureCandidates: []string{}, Gaps: []string{}, ExistingCitations: []string{}}
}

func (v *EvidenceSummary) UnmarshalJSON(data []byte) error {
	type plain EvidenceSummary
	seeded := plain(NewEvidenceSummary())
	if err := json.Unmarshal(data, &seeded); err != nil {
		return err
	}
	*v = EvidenceSummary(seeded)
	return nil
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

type FrameSet struct {
	Frames              []StoryFrame `json:"frames"`
	GenerationRationale string       `json:"generation_rationale"`
	Confident           bool         `json:"confident"`
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

func NewNoveltyScan() NoveltyScan {
	return NoveltyScan{ClosestTitles: []string{}, CollisionRisks: []string{}, PositioningOpenings: []string{}}
}

func (v *NoveltyScan) UnmarshalJSON(data []byte) error {
	type plain NoveltyScan
	seeded := plain(NewNoveltyScan())
	if err := json.Unmarshal(data, &seeded); err != nil {
		return err
	}
	*v = NoveltyScan(seeded)
	return nil
}

type PositioningDecision struct {
	WinningFrameName     string   `json:"winning_frame_name"`
	FinalTitle           string   `json:"final_title"`
	FinalAbstract        string   `json:"final_abstract"`
	OpeningThesis        string   `json:"opening_thesis"`
	ContributionOrder    []string `json:"contribution_order"`
	RejectedAlternatives []string `json:"rejected_alternatives"`
	SelectionRationale   string   `json:"selection_rationale"`
	Score                float64  `json:"score"`
	Confident            bool     `json:"confident"`
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

func NewSectionSpec() SectionSpec {
	return SectionSpec{EvidenceIDs: []string{}, FigureSlugs: []string{}, TargetWords: 400}
}

func (v *SectionSpec) UnmarshalJSON(data []byte) error {
	type plain SectionSpec
	seeded := plain(NewSectionSpec())
	if err := json.Unmarshal(data, &seeded); err != nil {
		return err
	}
	*v = SectionSpec(seeded)
	return nil
}

type FigureSpec struct {
	Index           int      `json:"index"`
	Slug            string   `json:"slug"`
	Purpose         string   `json:"purpose"`
	DataSources     []string `json:"data_sources"`
	Buildable       bool     `json:"buildable"`
	CaptionTakeaway string   `json:"caption_takeaway"`
}

func NewFigureSpec() FigureSpec { return FigureSpec{DataSources: []string{}} }

func (v *FigureSpec) UnmarshalJSON(data []byte) error {
	type plain FigureSpec
	seeded := plain(NewFigureSpec())
	if err := json.Unmarshal(data, &seeded); err != nil {
		return err
	}
	*v = FigureSpec(seeded)
	return nil
}

type Blueprint struct {
	Sections      []SectionSpec `json:"sections"`
	Figures       []FigureSpec  `json:"figures"`
	CitationNeeds []string      `json:"citation_needs"`
	VenueNotes    string        `json:"venue_notes"`
	Confident     bool          `json:"confident"`
}

func NewBlueprint() Blueprint {
	return Blueprint{Sections: []SectionSpec{}, Figures: []FigureSpec{}, CitationNeeds: []string{}}
}

func (v *Blueprint) UnmarshalJSON(data []byte) error {
	type plain Blueprint
	seeded := plain(NewBlueprint())
	if err := json.Unmarshal(data, &seeded); err != nil {
		return err
	}
	*v = Blueprint(seeded)
	return nil
}

type WorkerResult struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Summary string   `json:"summary"`
	Files   []string `json:"files"`
}

func NewWorkerResult() WorkerResult { return WorkerResult{Files: []string{}} }

func (v *WorkerResult) UnmarshalJSON(data []byte) error {
	type plain WorkerResult
	seeded := plain(NewWorkerResult())
	if err := json.Unmarshal(data, &seeded); err != nil {
		return err
	}
	*v = WorkerResult(seeded)
	return nil
}

type BuildReport struct {
	Sections     []WorkerResult `json:"sections"`
	Figures      []WorkerResult `json:"figures"`
	Bibliography WorkerResult   `json:"bibliography"`
	Confident    bool           `json:"confident"`
}

type CompileReport struct {
	Success      bool   `json:"success"`
	Attempts     int    `json:"attempts"`
	PDFPath      string `json:"pdf_path"`
	ErrorExcerpt string `json:"error_excerpt"`
}

type SlopViolation struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Rule    string `json:"rule"`
	Excerpt string `json:"excerpt"`
}

type SlopReport struct {
	Violations []SlopViolation `json:"violations"`
	Score      float64         `json:"score"`
}

func (r SlopReport) String() string {
	var out strings.Builder
	fmt.Fprintf(&out, "# Slop report\n\nScore: %.2f\n", r.Score)
	if len(r.Violations) == 0 {
		out.WriteString("\nNo violations.\n")
		return out.String()
	}
	out.WriteString("\n| File | Line | Rule | Excerpt |\n|---|---:|---|---|\n")
	for _, violation := range r.Violations {
		excerpt := strings.ReplaceAll(violation.Excerpt, "|", `\|`)
		fmt.Fprintf(&out, "| %s | %d | %s | %s |\n", violation.File, violation.Line, violation.Rule, excerpt)
	}
	return out.String()
}

type LocatedIssue struct {
	Section  string `json:"section"`
	Issue    string `json:"issue"`
	FixHint  string `json:"fix_hint"`
	Severity string `json:"severity"`
}

type PersonaReview struct {
	Persona        string         `json:"persona"`
	Issues         []LocatedIssue `json:"issues"`
	AcceptanceRisk float64        `json:"acceptance_risk"`
	Verdict        string         `json:"verdict"`
	Confident      bool           `json:"confident"`
}

type NarrativeReview struct {
	TransitionIssues       []LocatedIssue `json:"transition_issues"`
	PromiseAlignmentIssues []string       `json:"promise_alignment_issues"`
	ArcAssessment          string         `json:"arc_assessment"`
	Score                  float64        `json:"score"`
	Confident              bool           `json:"confident"`
}

type FidelityAudit struct {
	UnsupportedClaims []string `json:"unsupported_claims"`
	NumberMismatches  []string `json:"number_mismatches"`
	CitationIssues    []string `json:"citation_issues"`
	Blocking          bool     `json:"blocking"`
	Score             float64  `json:"score"`
	Confident         bool     `json:"confident"`
}

type CritiqueBundle struct {
	Round          int             `json:"round"`
	PersonaReviews []PersonaReview `json:"persona_reviews"`
	Narrative      NarrativeReview `json:"narrative"`
	Fidelity       FidelityAudit   `json:"fidelity"`
	Slop           SlopReport      `json:"slop"`
	Confident      bool            `json:"confident"`
}

type RepairTask struct {
	Target       string   `json:"target"`
	Instructions []string `json:"instructions"`
	Priority     string   `json:"priority"`
}

type RepairPlan struct {
	Tasks     []RepairTask `json:"tasks"`
	Notes     string       `json:"notes"`
	Confident bool         `json:"confident"`
}

type RoundRecord struct {
	Round          int     `json:"round"`
	TotalScore     float64 `json:"total_score"`
	PersonaScore   float64 `json:"persona_score"`
	NarrativeScore float64 `json:"narrative_score"`
	FidelityScore  float64 `json:"fidelity_score"`
	SlopScore      float64 `json:"slop_score"`
	CompileOK      bool    `json:"compile_ok"`
	RepairsApplied int     `json:"repairs_applied"`
	Stop           bool    `json:"stop"`
	StopReason     string  `json:"stop_reason"`
}

type WriteResult struct {
	Status          string        `json:"status"`
	RunID           string        `json:"run_id"`
	Workspace       string        `json:"workspace"`
	PDFPath         string        `json:"pdf_path"`
	Title           string        `json:"title"`
	Rounds          []RoundRecord `json:"rounds"`
	FinalScore      float64       `json:"final_score"`
	StopReason      string        `json:"stop_reason"`
	TODOPath        string        `json:"todo_path"`
	ReviewPath      string        `json:"review_path"`
	PositioningPath string        `json:"positioning_path"`
}

func NewWriteResult() WriteResult { return WriteResult{Rounds: []RoundRecord{}} }

func (v *WriteResult) UnmarshalJSON(data []byte) error {
	type plain WriteResult
	seeded := plain(NewWriteResult())
	if err := json.Unmarshal(data, &seeded); err != nil {
		return err
	}
	*v = WriteResult(seeded)
	return nil
}
