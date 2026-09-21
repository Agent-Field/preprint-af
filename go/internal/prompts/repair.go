package prompts

import (
	"fmt"
	"strings"
)

const RepairPlanSystem = "You are a repair planner turning reviewer findings into a bounded set of edit tasks. Emit at MOST 8 RepairTasks. Group by `target`: one task per section slug at most; use 'front_matter' for title/abstract/main.tex issues; 'figures' for figure problems; 'bibliography' for citation/refs issues. Every fidelity finding (unsupported claim, number mismatch, citation issue) MUST be covered by a task with priority 'high'. Slop violations attach to the task for their section as mechanical instructions that quote the file and line, for example 'line 34: replace the em dash with a comma'. Instructions must be concrete, executable edits ('rewrite the opening sentence to state the measured 3.2x speedup'), never judgments ('improve the flow'). If the critique contains nothing worth fixing (no major issues, no fidelity findings, only trivial residue), return an EMPTY tasks list — that signals convergence. Set `confident` truthfully."

func RepairPlanPrompt(compactCritiqueJSON string) (system, user string) {
	system = RepairPlanSystem
	user = "Critique digest (compacted CritiqueBundle):\n" + compactCritiqueJSON + "\n\nReturn a RepairPlan with tasks (target, instructions, priority), notes, confident."
	return system, user
}

func RepairTaskPrompt(promptFiles, instructions []string) string {
	numbered := make([]string, len(instructions))
	for i, instruction := range instructions {
		numbered[i] = fmt.Sprintf("%d. %s", i+1, instruction)
	}
	return fmt.Sprintf(
		"You may modify ONLY these files: %s.\n\nApply exactly these instructions:\n%s\n\nObey AGENTS.md. Never change evidence-backed numbers unless an instruction states the number contradicts EVIDENCE.md. Preserve \\todobox items unless an instruction resolves one.",
		strings.Join(promptFiles, ", "),
		strings.Join(numbered, "\n"),
	)
}
