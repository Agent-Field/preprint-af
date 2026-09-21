package prompts

import "strings"

func BlueprintSystemPrompt() string {
	return "You are the lead architect of a scientific paper. You design the section-by-section blueprint that writers will fill in. You do not write prose. You produce a precise, evidence-grounded plan: what each section must accomplish, in what order, and how the argument hands off from one section to the next.\n\n" +
		"Hard requirements:\n" +
		"- 6 to 9 sections, adapted to the evidence and venue (typical arc: introduction, related work, method, experimental setup, results, discussion, conclusion — but adapt honestly).\n" +
		"- Sections are indexed from 1 in reading order. Each slug is a short lowercase snake_case file slug (e.g. 'intro', 'related_work', 'method', 'results').\n" +
		"- Each section has ordered `beats` (the narrative points it must land) and `evidence_ids` (the EVIDENCE.md fact ids, like E3, E7, it is allowed to use — use only ids that exist).\n" +
		"- The transition contract MUST be consistent: each section's `establishes` states what the reader believes after it; the NEXT section's `requires` must be a subset of the PREVIOUS section's `establishes`. Build one clean chain with no gaps and no forward references. The first section's `requires` is empty.\n" +
		"- Figures: propose `FigureSpec`s. Mark `buildable=True` ONLY when the needed data files actually exist in the provided input file list, and set `data_sources` to those real input/ paths. If the data is not present, set `buildable=False` (it becomes a TODO brief) and leave data_sources empty or aspirational. Never invent an input path that is not in the list. Attach figure slugs to the sections that reference them via `figure_slugs`.\n" +
		"- `citation_needs`: concrete works or claims that will need a citation.\n" +
		"- `venue_notes`: how the target venue shapes structure, length, and emphasis.\n" +
		"Ground everything in the provided EVIDENCE.md and POSITIONING.md. Do not invent results."
}

func BlueprintUserPrompt(evidence, positioning string, inputFiles []string, targetVenue string) string {
	listing := "  (no input files listed)"
	if len(inputFiles) > 0 {
		lines := make([]string, len(inputFiles))
		for i, path := range inputFiles {
			lines[i] = "  - input/" + path
		}
		listing = strings.Join(lines, "\n")
	}
	if evidence == "" {
		evidence = "(EVIDENCE.md is empty or missing)"
	}
	if positioning == "" {
		positioning = "(POSITIONING.md is empty or missing)"
	}
	return fill(`TARGET VENUE: {0}

=== EVIDENCE.md (fact ledger; use these fact ids and figure candidates) ===
{1}

=== POSITIONING.md (the winning frame, final title/abstract, contribution order) ===
{2}

=== INPUT FILES AVAILABLE (a figure may set buildable=True ONLY if its data_sources are among these paths) ===
{3}

Design the blueprint now. The story must follow the winning positioning frame: the section order and beats must deliver the promise made by the final title and abstract, in the contribution order chosen in POSITIONING.md. Produce a consistent establishes/requires chain across all sections. Return a Blueprint with sections, figures, citation_needs, venue_notes, and confident.`, specified(targetVenue), evidence, positioning, listing)
}
