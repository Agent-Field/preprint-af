package prompts

func LatexRepairPrompt(excerpt string) string {
	return `LaTeX compilation failed. Fix ONLY LaTeX/compilation errors: undefined control sequences, missing
packages (prefer removing the dependency), math-mode errors, unescaped %, &, #, _ in text, missing
\includegraphics files (replace with \todobox{Figure <name> pending} if the pdf is genuinely
absent), bib issues. Do NOT change scientific content, numbers, citations, prose wording, or
section structure. Error log: ` + excerpt
}
