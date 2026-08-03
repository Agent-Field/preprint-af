package prompts

import "strings"

var PositioningPersonas = []string{
	"a domain expert who checks whether the central result is technically meaningful and correctly interpreted, not just impressive-sounding",
	"a skeptical methods reviewer hunting for overclaims, unsupported evaluation, and any gap between what the title promises and what the evidence actually shows",
	"an interdisciplinary editor judging whether the first page signals broad importance to readers outside the immediate subfield",
	"a busy area chair judging how easily this title and abstract can be placed in a crowded program and defended against competing submissions",
}

func FrameGenerationPrompts(targetVenue, fieldHint, evidence string) (system, user string) {
	system = "You are a scientific positioning strategist deciding how to frame a paper for maximum HONEST impact. The same evidence can be told as several genuinely different stories, speed, memory, mechanism, reliability, benchmark, or methodological insight, and your job is to surface the distinct options a top scientist would actually weigh. Judges score concrete artifacts, not descriptions, so every frame you propose must carry a real candidate title and a real mini-abstract written in that frame. You never invent numbers, datasets, or results, and you refuse to propose a frame the evidence cannot support."
	user = fill(`Target venue: {0}
Field hint: {1}

Full evidence ledger (EVIDENCE.md):
-----------------------------------
{2}
-----------------------------------

Produce 5 to 6 GENUINELY different frames for this paper. Each frame must commit to a different §angle§, the ONE thing it makes primary, drawn from options such as speed, memory, mechanism, reliability, benchmark, or methodological insight, whichever the evidence honestly supports. Frames must differ in what they make primary, not merely in wording.

For each frame supply:
- name: a short internal label for the frame.
- angle: the single dimension it foregrounds.
- central_thesis: the one claim the whole paper would defend.
- title: a CONCRETE candidate title written in this frame (not a description of one), reading like a senior human scientist wrote it.
- mini_abstract: a CONCRETE abstract of about 120 words written fully in this frame, carrying problem, precise gap, approach, the main quantitative result stated with the REAL numbers from the evidence, and the implication.
- contribution_order: the ordered list of contributions this frame leads with.
- figure_emphasis: which figures or plots this frame would foreground.
- why_it_could_win: the honest case for this framing.
- risk_of_failure: the honest way this framing could collapse under review.
- evidence_alignment and impact_potential: honest scores in [0,1].

Hard rules: do NOT invent any number, dataset, metric, or result that is not already in the evidence ledger. Do NOT propose any frame the evidence cannot support, if there is no speed result, do not write a speed frame. Set §confident§ true only if the evidence is rich enough to position the paper honestly, and put your reasoning about the spread of frames in §generation_rationale§.`, specified(targetVenue), specified(fieldHint), evidence)
	return
}

func JudgeFramePrompts(frame StoryFrame, evidenceDigest, persona, targetVenue string) (system, user string) {
	system = "You are simulating ONE specific senior scientific reader judging a single candidate framing of a paper. You see the frame's concrete title, its central thesis, and its ~120-word mini-abstract, plus a digest of the paper's evidence. Score it honestly on four axes and raise concrete concerns. Naturalness matters as much as content: a real senior scientist writes plainly. Penalize colon-stacking (Title: Subtitle: Sub-subtitle), keyword packing, hype adjectives (novel, powerful, comprehensive, groundbreaking, seamless), generic 'novel framework' phrasing, and symmetric slogan rhythm that reads machine-generated."
	name := frame.Name
	if name == "" {
		name = "unnamed frame"
	}
	user = fill(`You are {0}.
Target venue: {1}

Frame under review
------------------
name: {2}
angle: {3}
title: {4}
central_thesis: {5}
mini_abstract: {6}

Evidence digest
---------------
{7}

Score each axis in [0,1] from your persona's stance:
- comprehension: can you grasp the actual contribution from the title and abstract alone?
- excitement: does this make you want to read the paper?
- credibility: is every claim in the title and abstract backed by the evidence digest, with no overclaim?
- naturalness: would a senior human scientist actually write this title and abstract, or does it read as generated?

List CONCRETE concerns: quote the exact word, phrase, or claim that triggers each concern. No generic complaints. Set §confident§ honestly.`, persona, specified(targetVenue), name, frame.Angle, frame.Title, frame.CentralThesis, frame.MiniAbstract, evidenceDigest)
	return
}

func NoveltyScanPrompt(workspaceRoot string, frames []StoryFrame) string {
	lines := make([]string, len(frames))
	for i, frame := range frames {
		lines[i] = "- " + strings.TrimSpace(frame.Title) + "  ||  thesis: " + strings.TrimSpace(frame.CentralThesis)
	}
	rendered := "- (no frames provided)"
	if len(lines) > 0 {
		rendered = strings.Join(lines, "\n")
	}
	return fill(`You are a scientific literature scout with web access. We are choosing how to position a paper and need to know what already exists nearby.

Frames under consideration (candidate title || central thesis):
{0}

Task:
1. Use web search across arXiv, Semantic Scholar, and Google Scholar to find the CLOSEST real related work to these frame titles and theses.
2. Write a file named POSITIONING_SCAN.md at the root of this workspace ({1}) containing:
   - Closest real titles found, quoted VERBATIM, each with a working link (arXiv id, DOI, or URL).
   - How each of those papers positions itself (its angle and main claim).
   - Collision risks for EACH of our frames: which existing paper would a reviewer say we overlap with, and how badly.
   - Open positioning angles that nearby work leaves unclaimed.
3. NEVER fabricate a paper, author, title, or link. If a search returns nothing usable, or web access fails, say so explicitly in the file rather than inventing anything.

Then return the structured NoveltyScan: closest_titles (verbatim), collision_risks, positioning_openings, and confident.`, rendered, workspaceRoot)
}

func MetaSelectionPrompts(targetVenue, fieldHint string, frames []StoryFrame, judgments []FrameJudgment, novelty NoveltyScan) (system, user string) {
	system = "You are the meta-editor making the final positioning decision for this paper. You are given several candidate frames (each with a concrete title and mini-abstract), the judgments of four senior readers on each frame, and a novelty scan of nearby published work. Pick exactly ONE winning frame. Do NOT average the scores: a frame with high excitement but weak credibility must lose, and a frame that collides with existing work should be discounted. Then write the paper's real front matter, obeying the hard constraints below, in the plain voice of a senior scientist."
	user = fill(`Target venue: {0}
Field hint: {1}

Candidate frames:
{2}

Reader judgments (four personas per frame):
{3}

Novelty scan of nearby work:
{4}

Decide and produce:
- winning_frame_name: the §name§ of the single frame that wins.
- final_title: obey ALL of these hard rules, at most 14 words, at most one colon, no question mark, no keyword stuffing, and it must read like a senior human wrote it.
- final_abstract: a FULL abstract of about 180 words (NOT the mini-abstract), following the arc problem, then the precise gap, then the approach, then the main quantitative result stated with the REAL numbers drawn from the frames and evidence, then the implication. Invent NO number that is not already present in the frames.
- opening_thesis: the single sentence the introduction should open on.
- contribution_order: the ordered list of contributions the paper will make.
- rejected_alternatives: one entry per losing frame, each formatted as '<frame name>: <one-line reason it lost>'.
- selection_rationale: why the winner beats the rest, grounded in credibility and novelty, not just excitement.
- score in [0,1] and confident.
`, specified(targetVenue), specified(fieldHint), PythonRepr(frames), PythonRepr(judgments), PythonRepr(novelty))
	return
}
