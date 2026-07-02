from __future__ import annotations

from pydantic import BaseModel, Field


class WriteRequest(BaseModel):
    folder_path: str = Field(description="Folder with the user's research: data, results, drafts, or an existing paper.")
    target_venue: str | None = Field(default=None, description="Target journal/conference, e.g. 'NeurIPS' or 'Nature Communications'.")
    field_hint: str | None = Field(default=None, description="Field, e.g. 'machine learning systems'.")
    max_rounds: int = Field(default=3, ge=1, le=8, description="Cap on critique/repair rounds after the first full build.")
    allow_web: bool = Field(default=True, description="Allow web lookups for citations and positioning scans.")
    dry_run: bool = Field(default=False, description="Stop after evidence, positioning, and blueprint; write no paper.")
    quality_threshold: float = Field(default=0.90, ge=0.0, le=1.0)
    plateau_delta: float = Field(default=0.01, ge=0.0, le=0.1)
    model: str | None = None


class Workspace(BaseModel):
    run_id: str
    root: str
    input_dir: str
    paper_dir: str
    sections_dir: str
    figures_dir: str
    reviews_dir: str
    evidence_path: str
    positioning_path: str
    blueprint_path: str
    todo_path: str
    source_folder: str
    input_files: list[str]
    has_existing_draft: bool
    has_data_files: bool


class EvidenceSummary(BaseModel):
    fact_count: int = 0
    figure_candidates: list[str] = Field(default_factory=list)
    gaps: list[str] = Field(default_factory=list, description="Missing experiments or data, phrased as TODO items.")
    existing_citations: list[str] = Field(default_factory=list)
    draft_assessment: str = ""
    strongest_factual_thesis: str = ""
    confident: bool = False


class StoryFrame(BaseModel):
    name: str
    angle: str = Field(description="What the frame makes primary: e.g. speed, memory, mechanism, benchmark, reliability.")
    central_thesis: str
    title: str = Field(description="Concrete candidate title written in this frame.")
    mini_abstract: str = Field(description="Concrete ~120-word candidate abstract written in this frame.")
    contribution_order: list[str]
    figure_emphasis: list[str]
    why_it_could_win: str
    risk_of_failure: str
    evidence_alignment: float = Field(ge=0.0, le=1.0)
    impact_potential: float = Field(ge=0.0, le=1.0)


class FrameSet(BaseModel):
    frames: list[StoryFrame]
    generation_rationale: str
    confident: bool


class FrameJudgment(BaseModel):
    frame_name: str
    persona: str
    comprehension: float = Field(ge=0.0, le=1.0)
    excitement: float = Field(ge=0.0, le=1.0)
    credibility: float = Field(ge=0.0, le=1.0)
    naturalness: float = Field(ge=0.0, le=1.0, description="Does the title/abstract read like a top human scientist wrote it?")
    concerns: list[str]
    confident: bool


class NoveltyScan(BaseModel):
    closest_titles: list[str] = Field(default_factory=list)
    collision_risks: list[str] = Field(default_factory=list)
    positioning_openings: list[str] = Field(default_factory=list, description="Angles nearby papers leave open.")
    confident: bool = False


class PositioningDecision(BaseModel):
    winning_frame_name: str
    final_title: str
    final_abstract: str
    opening_thesis: str
    contribution_order: list[str]
    rejected_alternatives: list[str]
    selection_rationale: str
    score: float = Field(ge=0.0, le=1.0)
    confident: bool


class SectionSpec(BaseModel):
    index: int
    slug: str = Field(description="File slug, e.g. 'intro' -> sections/01_intro.tex.")
    heading: str
    beats: list[str] = Field(description="Ordered narrative beats this section must land.")
    establishes: str = Field(description="What the reader knows/believes after this section (contract for the next).")
    requires: str = Field(default="", description="What the previous section must have established.")
    evidence_ids: list[str] = Field(default_factory=list, description="EVIDENCE.md fact ids this section may use.")
    figure_slugs: list[str] = Field(default_factory=list)
    target_words: int = 400


class FigureSpec(BaseModel):
    index: int
    slug: str
    purpose: str = Field(description="The single takeaway the figure must show.")
    data_sources: list[str] = Field(default_factory=list, description="Files under input/ the figure is built from.")
    buildable: bool = Field(description="False when required data is missing; becomes a TODO brief instead.")
    caption_takeaway: str = ""


class Blueprint(BaseModel):
    sections: list[SectionSpec]
    figures: list[FigureSpec]
    citation_needs: list[str] = Field(default_factory=list)
    venue_notes: str = ""
    confident: bool


class WorkerResult(BaseModel):
    name: str
    status: str = Field(description="done | failed | skipped | todo")
    summary: str = ""
    files: list[str] = Field(default_factory=list)


class BuildReport(BaseModel):
    sections: list[WorkerResult]
    figures: list[WorkerResult]
    bibliography: WorkerResult
    confident: bool


class CompileReport(BaseModel):
    success: bool
    attempts: int
    pdf_path: str = ""
    error_excerpt: str = ""


class SlopViolation(BaseModel):
    file: str
    line: int
    rule: str
    excerpt: str


class SlopReport(BaseModel):
    violations: list[SlopViolation]
    score: float = Field(ge=0.0, le=1.0)


class LocatedIssue(BaseModel):
    section: str = Field(description="Section slug or 'front_matter' or 'global'.")
    issue: str
    fix_hint: str
    severity: str = Field(description="major | minor")


class PersonaReview(BaseModel):
    persona: str
    issues: list[LocatedIssue]
    acceptance_risk: float = Field(ge=0.0, le=1.0)
    verdict: str
    confident: bool


class NarrativeReview(BaseModel):
    transition_issues: list[LocatedIssue]
    promise_alignment_issues: list[str] = Field(description="Where the body under-delivers or over-delivers vs title/abstract.")
    arc_assessment: str
    score: float = Field(ge=0.0, le=1.0)
    confident: bool


class FidelityAudit(BaseModel):
    unsupported_claims: list[str]
    number_mismatches: list[str]
    citation_issues: list[str]
    blocking: bool = Field(description="True when claims drift beyond EVIDENCE.md and the round must not pass.")
    score: float = Field(ge=0.0, le=1.0)
    confident: bool


class CritiqueBundle(BaseModel):
    round: int
    persona_reviews: list[PersonaReview]
    narrative: NarrativeReview
    fidelity: FidelityAudit
    slop: SlopReport
    confident: bool


class RepairTask(BaseModel):
    target: str = Field(description="Section slug, 'front_matter', 'figures', or 'bibliography'.")
    instructions: list[str]
    priority: str = Field(description="high | medium | low")


class RepairPlan(BaseModel):
    tasks: list[RepairTask]
    notes: str = ""
    confident: bool


class RoundRecord(BaseModel):
    round: int
    total_score: float
    persona_score: float
    narrative_score: float
    fidelity_score: float
    slop_score: float
    compile_ok: bool
    repairs_applied: int
    stop: bool
    stop_reason: str


class WriteResult(BaseModel):
    status: str
    run_id: str
    workspace: str
    pdf_path: str = ""
    title: str = ""
    rounds: list[RoundRecord] = Field(default_factory=list)
    final_score: float = 0.0
    stop_reason: str = ""
    todo_path: str = ""
    review_path: str = ""
    positioning_path: str = ""
