from __future__ import annotations

from agentfield import AgentRouter

from . import helpers
from .models import CompileReport

router = AgentRouter(prefix="latex", tags=["latex"])

MAX_ATTEMPTS = 3


def _repair_prompt(excerpt: str) -> str:
    return f"""\
LaTeX compilation failed. Fix ONLY LaTeX/compilation errors: undefined control sequences, missing
packages (prefer removing the dependency), math-mode errors, unescaped %, &, #, _ in text, missing
\\includegraphics files (replace with \\todobox{{Figure <name> pending}} if the pdf is genuinely
absent), bib issues. Do NOT change scientific content, numbers, citations, prose wording, or
section structure. Error log: {excerpt}"""


@router.reasoner()
async def compile_paper(workspace: dict, model: str | None = None) -> CompileReport:
    """P4: compile the paper with latexmk, repairing LaTeX errors between attempts (up to 3)."""
    root = workspace["root"]
    paper_dir = workspace["paper_dir"]

    last_excerpt = ""
    for attempt in range(1, MAX_ATTEMPTS + 1):
        ok, pdf_path, excerpt = helpers.run_latexmk(paper_dir)
        if ok:
            print(f"[latex] compile succeeded on attempt {attempt}: {pdf_path}")
            helpers.git_snapshot(root, f"P4 compile success (attempt {attempt})")
            return CompileReport(success=True, attempts=attempt, pdf_path=pdf_path)

        last_excerpt = excerpt
        print(f"[latex] compile attempt {attempt} failed")
        if attempt >= MAX_ATTEMPTS:
            break

        # One repair pass targeting only LaTeX/compilation errors.
        result = await router.harness(
            _repair_prompt(excerpt),
            provider="opencode",
            model=helpers.opencode_model(model),
            cwd=paper_dir,
            project_dir=root,
        )
        if result.is_error:
            print(f"[latex] repair pass (after attempt {attempt}) errored: {result.error_message}")

    print(f"[latex] compile failed after {MAX_ATTEMPTS} attempts")
    return CompileReport(success=False, attempts=MAX_ATTEMPTS, error_excerpt=last_excerpt)
