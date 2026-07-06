from __future__ import annotations

import json
import os
import re
import shutil
import statistics
import subprocess
import sys
import time
import types
import uuid
from pathlib import Path
from typing import Union, get_args, get_origin

from .models import SlopReport, SlopViolation, Workspace

DEFAULT_MODEL = "openrouter/deepseek/deepseek-v4-pro"

SLUG = "preprint-af"

# Directories never copied into the workspace input mirror.
_SKIP_DIRS = {".git", ".venv", "__pycache__", "node_modules"}
# Files larger than this are skipped during the input copy.
_MAX_FILE_BYTES = 100 * 1024 * 1024
_DATA_EXTS = {".csv", ".json", ".parquet", ".npz", ".npy", ".pkl", ".h5", ".jsonl"}
_INPUT_FILE_CAP = 400

_GIT_IDENTITY = [
    "-c",
    "user.email=paper-improver@local",
    "-c",
    "user.name=paper-improver",
]


# --------------------------------------------------------------------------- #
# Identity / models                                                           #
# --------------------------------------------------------------------------- #


def node_id() -> str:
    return os.getenv("AGENT_NODE_ID", SLUG)


def project_root() -> Path:
    """Repo root — where `.tmp/runs/<id>` workspaces live.

    Anchored on a repo marker (pyproject.toml / .git) so it stays correct regardless of
    how deep this module sits under `src/`. Falls back to three levels up
    (src/preprint_af/reasoners/ -> repo root) if no marker is found.
    """
    here = Path(__file__).resolve()
    for parent in here.parents:
        if (parent / "pyproject.toml").exists() or (parent / ".git").exists():
            return parent
    return here.parents[3]


def ai_model(model: str | None) -> str:
    return model or os.getenv("AI_MODEL") or DEFAULT_MODEL


def opencode_model(model: str | None) -> str:
    return model or os.getenv("OPENCODE_MODEL") or DEFAULT_MODEL


def figure_python() -> str:
    return sys.executable


# --------------------------------------------------------------------------- #
# Filesystem helpers                                                          #
# --------------------------------------------------------------------------- #


def read_text(path: str | Path, limit: int | None = None) -> str:
    try:
        text = Path(path).read_text(encoding="utf-8", errors="ignore")
    except OSError:
        return ""
    return text[:limit] if limit is not None else text


def write_text(path: str | Path, content: str) -> str:
    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")
    return str(target)


def append_todos(todo_path: str, items: list[str]) -> None:
    """Append new `- ` bullets to TODO.md, deduplicating by exact line."""
    existing = read_text(todo_path)
    seen = {line.strip() for line in existing.splitlines()}
    new_bullets: list[str] = []
    for raw in items:
        item = raw.strip()
        if not item:
            continue
        bullet = item if item.startswith("- ") else f"- {item}"
        if bullet in seen or item in seen:
            continue
        seen.add(bullet)
        new_bullets.append(bullet)
    if not new_bullets:
        return
    content = existing
    if content and not content.endswith("\n"):
        content += "\n"
    content += "\n".join(new_bullets) + "\n"
    write_text(todo_path, content)


def assemble_paper_text(paper_dir: str) -> str:
    """main.tex then sorted sections/*.tex, each prefixed with a `--- <relpath> ---` marker."""
    paper = Path(paper_dir)
    parts: list[str] = []
    main = paper / "main.tex"
    if main.exists():
        parts.append(f"--- main.tex ---\n{read_text(main)}")
    sections = paper / "sections"
    if sections.is_dir():
        for sec in sorted(sections.glob("*.tex")):
            rel = sec.relative_to(paper).as_posix()
            parts.append(f"--- {rel} ---\n{read_text(sec)}")
    return "\n\n".join(parts)


def save_state(root: str, state: dict) -> None:
    write_text(Path(root) / "STATE.json", json.dumps(state, indent=2))


def load_state(root: str) -> dict:
    text = read_text(Path(root) / "STATE.json")
    if not text:
        return {}
    try:
        return json.loads(text)
    except (ValueError, TypeError):
        return {}


# --------------------------------------------------------------------------- #
# Git (best-effort; never raises)                                             #
# --------------------------------------------------------------------------- #


def _git(root: str | Path, args: list[str], timeout: int = 60) -> subprocess.CompletedProcess | None:
    if shutil.which("git") is None:
        return None
    try:
        return subprocess.run(
            ["git", *args],
            cwd=str(root),
            capture_output=True,
            text=True,
            timeout=timeout,
        )
    except (OSError, subprocess.SubprocessError):
        return None


def git_snapshot(root: str, label: str) -> None:
    _git(root, ["add", "-A"])
    _git(root, [*_GIT_IDENTITY, "commit", "-m", label])


def git_changed_files(root: str) -> list[str]:
    result = _git(root, ["status", "--porcelain"], timeout=30)
    if result is None or result.returncode != 0:
        return []
    files: list[str] = []
    for line in result.stdout.splitlines():
        if len(line) > 3:
            path = line[3:].strip()
            if "->" in path:  # rename: "old -> new"
                path = path.split("->", 1)[1].strip()
            if path:
                files.append(path)
    return files


# --------------------------------------------------------------------------- #
# Workspace creation                                                          #
# --------------------------------------------------------------------------- #


def create_workspace(folder_path: str) -> Workspace:
    source = Path(folder_path).expanduser().resolve()
    run_id = uuid.uuid4().hex[:12]
    root = project_root() / ".tmp" / "runs" / run_id
    input_dir = root / "input"
    paper_dir = root / "paper"
    sections_dir = paper_dir / "sections"
    figures_dir = paper_dir / "figures"
    reviews_dir = root / "reviews"

    for directory in (input_dir, sections_dir, figures_dir, reviews_dir):
        directory.mkdir(parents=True, exist_ok=True)

    _copy_input(source, input_dir)

    write_text(root / "AGENTS.md", style_contract_markdown())
    write_text(root / "TODO.md", "")

    _git(root, ["init"], timeout=30)
    git_snapshot(str(root), "initial workspace")

    return Workspace(
        run_id=run_id,
        root=str(root),
        input_dir=str(input_dir),
        paper_dir=str(paper_dir),
        sections_dir=str(sections_dir),
        figures_dir=str(figures_dir),
        reviews_dir=str(reviews_dir),
        evidence_path=str(root / "EVIDENCE.md"),
        positioning_path=str(root / "POSITIONING.md"),
        blueprint_path=str(root / "BLUEPRINT.md"),
        todo_path=str(root / "TODO.md"),
        source_folder=str(source),
        input_files=_list_input_files(input_dir),
        has_existing_draft=_detect_existing_draft(input_dir),
        has_data_files=_detect_data_files(input_dir),
    )


def _copy_input(source: Path, input_dir: Path) -> None:
    if not source.exists() or not source.is_dir():
        return
    for dirpath, dirnames, filenames in os.walk(source):
        dirnames[:] = [d for d in dirnames if d not in _SKIP_DIRS]
        rel = Path(dirpath).relative_to(source)
        target_dir = input_dir / rel
        target_dir.mkdir(parents=True, exist_ok=True)
        for name in filenames:
            src = Path(dirpath) / name
            try:
                if src.is_symlink() or not src.is_file():
                    continue
                if src.stat().st_size > _MAX_FILE_BYTES:
                    continue
                shutil.copy2(src, target_dir / name)
            except OSError:
                continue


def _list_input_files(input_dir: Path) -> list[str]:
    files: list[str] = []
    for path in sorted(input_dir.rglob("*")):
        if path.is_file():
            files.append(path.relative_to(input_dir).as_posix())
            if len(files) >= _INPUT_FILE_CAP:
                break
    return files


def _detect_existing_draft(input_dir: Path) -> bool:
    for path in input_dir.rglob("*"):
        if not path.is_file() or path.suffix.lower() not in {".tex", ".md"}:
            continue
        text = read_text(path)
        if len(text) <= 2000:
            continue
        if path.suffix.lower() == ".md" and re.search(r"^#{1,6}\s+\S", text, re.MULTILINE):
            return True
        if path.suffix.lower() == ".tex" and re.search(r"\\(?:sub)*section\*?\{", text):
            return True
    return False


def _detect_data_files(input_dir: Path) -> bool:
    for path in input_dir.rglob("*"):
        if not path.is_file():
            continue
        suffix = path.suffix.lower()
        if suffix in _DATA_EXTS:
            return True
        if suffix == ".py":
            snippet = read_text(path, 4000).lower()
            if "matplotlib" in snippet or "savefig" in snippet or "plt." in snippet:
                return True
    return False


# --------------------------------------------------------------------------- #
# LaTeX compilation                                                           #
# --------------------------------------------------------------------------- #


def run_latexmk(paper_dir: str, timeout_s: int = 420) -> tuple[bool, str, str]:
    paper = Path(paper_dir)
    pdf = paper / "main.pdf"
    if shutil.which("latexmk") is None:
        return (False, "", "latexmk not found on PATH")

    before = pdf.stat().st_mtime if pdf.exists() else None
    try:
        proc = subprocess.run(
            ["latexmk", "-pdf", "-interaction=nonstopmode", "-f", "main.tex"],
            cwd=str(paper),
            capture_output=True,
            text=True,
            timeout=timeout_s,
        )
    except FileNotFoundError:
        return (False, "", "latexmk not found on PATH")
    except subprocess.TimeoutExpired:
        excerpt = _latex_log_excerpt(paper) or "latexmk timed out"
        return (False, str(pdf) if pdf.exists() else "", excerpt)

    modified = pdf.exists() and (before is None or pdf.stat().st_mtime > before)
    success = proc.returncode == 0 or modified
    pdf_path = str(pdf) if pdf.exists() else ""
    if success:
        return (True, pdf_path, "")
    return (False, pdf_path, _latex_log_excerpt(paper))


def _latex_log_excerpt(paper: Path) -> str:
    text = read_text(paper / "main.log")
    if not text:
        return ""
    matched = [
        line
        for line in text.splitlines()
        if "!" in line or "Error" in line or "Undefined" in line
    ]
    return "\n".join(matched[-60:])[:4000]


# --------------------------------------------------------------------------- #
# Style contract (AGENTS.md) and main.tex skeleton                            #
# --------------------------------------------------------------------------- #


def style_contract_markdown() -> str:
    return """# AGENTS.md — paper style and fidelity contract

You are writing or editing scientific-paper artifacts inside this workspace. Follow
these rules exactly. They are read automatically for every task.

## 1. Role

- You write or edit ONE artifact of a scientific paper at a time.
- Touch ONLY the file(s) your prompt assigns. Never edit other sections, main.tex,
  the bibliography, figures, or the evidence/positioning/blueprint documents unless
  the prompt names them as your writable target.

## 2. Fidelity rules

- Every number, metric, and factual claim must trace to an EVIDENCE.md fact id. Put the
  `% E<n>` comment on its OWN line, immediately after the sentence that uses the fact.
  NEVER append `%` to a line that has more text after it: in LaTeX, everything after `%`
  on that line silently disappears from the compiled paper.
- Never invent results, datasets, metrics, citations, or figures.
- Missing material becomes `\\todobox{what is needed and why}`; `\\todobox` renders as
  inline red text that flows with the prose — do not fabricate to fill a gap.
- Cite only keys present in `paper/refs.bib`. A needed but unknown citation becomes a `\\todobox`.

## 3. Prose rules (hard bans)

- NO em dashes (—) and no ` --- ` in prose. Use commas, periods, or parentheses.
- Banned words and phrases: "delve", "showcase", "underscore" / "underscores", "pivotal",
  "crucial role", "paradigm", "cutting-edge", "comprehensive framework", "novel approach"
  (as a phrase), "it is worth noting", "in conclusion".
- "moreover" at most once per section. "furthermore" at most twice per paper.
  "leverage" / "leveraging" at most once per paper.
- No "not only ... but also" constructions.
- No rhetorical questions. No exclamation marks.

## 4. Prose doctrine (how to write, not just what to avoid)

- Mechanism before formalism. Say the idea in plain words first; the equation then
  arrives as the inevitable formalization of what was just said. Every displayed
  equation is part of a sentence, punctuated, introduced by need ("To capture X, we
  define ...") and followed by one sentence interpreting the term that matters.
  Never place two displayed equations without prose between them.
- The skim test. Reading only the first sentence of every paragraph must yield a
  coherent summary of the paper. Topic sentences carry claims, not setup.
- Old before new. Open each sentence with what the reader already knows and end with
  the new item. The end of a sentence is its stress position: put the payload there.
- Understatement backed by enormity. Let the result carry the weight and strip the
  adjectives. "This halves the cost of X" needs no "remarkably".
- Numbers with anchors. A number never appears without its comparison ("3.2x faster
  than the strongest baseline"). Headline numbers appear at most three times in the
  paper: abstract, results, conclusion.
- Write at the strength the evidence supports, then state it plainly. Calibrated
  confidence means scoping the claim, not hedging the sentence. "Our analysis assumes
  X" in Methods beats "unfortunately this does not work for X" in Results.
- Vary sentence length deliberately: mix short declaratives with dense technical
  sentences. Vary paragraph length too; uniform rhythm reads as machine-written.
- Transitions carry scientific content, never signposts. Never write "In this
  section, we ...".

## 5. Salience and placement (what goes where)

Every fact has a salience class; place it only where its class allows:

| Class | Examples | Allowed placement |
|---|---|---|
| headline | the central result | abstract, intro, results, captions, conclusion |
| supporting | per-benchmark numbers, ablations | results, near their figure |
| provenance | seeds, N, hardware, hyperparameters | Methods (once) or SI; never in the narrative |
| limitation | scope boundaries, failure regimes | the Limitations paragraph (once), or one clause in Methods |

- State each limitation exactly once, in the Limitations paragraph or Methods. Never
  interrupt the narrative to disclaim. Routine rigor (seeds, error bars, hardware) is
  stated once in Methods; repeating it elsewhere signals insecurity, not honesty.
- Banned defensive constructions: "unfortunately", "we were unable", "failed to"
  (about your own method, outside Limitations), "it should be noted", "we
  acknowledge that", "we note that our method does not".

### Before/after examples

Pedantic (wrong):
  "We note that our method does not extend to X, and we only validated with 4 seeds."
Correct:
  Results state the claim plainly. Methods says "all results are means over 4 seeds."
  Limitations says "our analysis assumes X; extending beyond it is future work."

Defensive (wrong):
  "Although our approach unfortunately fails in the high-noise regime, it achieves ..."
Correct:
  "Our approach achieves ... in the moderate-noise regimes that dominate practice."
  (High-noise behavior goes to Limitations, once.)

## 6. Structure rules

- No `\\subsubsection`.
- Use subsections only for real method or experiment families.
- Use bullets only for genuine enumerations (contributions, assumptions, datasets).
- Never end a section with a summary of itself.

## 7. Front matter rules

- Title: at most 14 words, at most one colon, no question mark, no keyword stuffing.
- Abstract: one paragraph with the arc problem -> gap -> approach -> main quantitative
  result -> implication. No bullet-like sentence lists.

## 8. LaTeX rules

- Sections live in `paper/sections/NN_slug.tex` and contain no `\\documentclass`.
- Reference figures as `figures/<slug>.pdf` with `\\label{fig:<slug>}`.
- Captions state the takeaway, not what the axes are.
- The `\\todobox{...}` command is defined in main.tex; do not redefine it.
- Keep the package set minimal.
"""


def _escape_front_matter(text: str) -> str:
    """Escape only %, & and # for use inside \\title / abstract."""
    return text.replace("%", r"\%").replace("&", r"\&").replace("#", r"\#")


def _input_stem(section_file: str) -> str:
    name = section_file.strip()
    if name.endswith(".tex"):
        name = name[:-4]
    if name.startswith("sections/"):
        name = name[len("sections/") :]
    return name


def main_tex_skeleton(title: str, abstract: str, section_files: list[str]) -> str:
    title_tex = _escape_front_matter(title)
    abstract_tex = _escape_front_matter(abstract)
    inputs = "\n".join(f"\\input{{sections/{_input_stem(sf)}}}" for sf in section_files)
    return f"""\\documentclass[11pt]{{article}}
\\usepackage[margin=1in]{{geometry}}
\\usepackage{{amsmath}}
\\usepackage{{amssymb}}
\\usepackage{{graphicx}}
\\graphicspath{{{{figures/}}}}
\\usepackage{{booktabs}}
\\usepackage{{xcolor}}
\\usepackage[numbers,sort&compress]{{natbib}}
\\usepackage{{hyperref}}

\\newcommand{{\\todobox}}[1]{{\\textcolor{{red}}{{\\textbf{{[TODO:}} #1\\textbf{{]}}}}}}

\\title{{{title_tex}}}
\\author{{Author Name\\thanks{{Draft generated for author revision.}}}}
\\date{{}}

\\begin{{document}}
\\maketitle

\\begin{{abstract}}
{abstract_tex}
\\end{{abstract}}

{inputs}

\\bibliographystyle{{plainnat}}
\\bibliography{{refs}}

\\end{{document}}
"""


# --------------------------------------------------------------------------- #
# Deterministic slop linter                                                   #
# --------------------------------------------------------------------------- #

# (rule-id token, regex) — each occurrence is flagged.
_HARD_BANS: list[tuple[str, str]] = [
    ("delve", r"\bdelve\b"),
    ("showcase", r"\bshowcase(?:s|d|ing)?\b"),
    ("underscore", r"\bunderscore(?:s|d)?\b"),
    ("pivotal", r"\bpivotal\b"),
    ("crucial role", r"\bcrucial role\b"),
    ("paradigm", r"\bparadigm\b"),
    ("cutting-edge", r"\bcutting[- ]edge\b"),
    ("comprehensive framework", r"\bcomprehensive framework\b"),
    ("novel approach", r"\bnovel approach\b"),
    ("it is worth noting", r"\bit is worth noting\b"),
    ("in conclusion", r"\bin conclusion\b"),
]

# (rule-id token, regex, scope, allowance) — flagged only beyond the allowance.
_THRESHOLD_BANS: list[tuple[str, str, str, int]] = [
    ("moreover", r"\bmoreover\b", "file", 1),
    ("furthermore", r"\bfurthermore\b", "paper", 2),
    ("leverage", r"\bleverag(?:e|es|ed|ing)\b", "paper", 1),
]

_BULLET_SLUG_MARKERS = ("intro", "discussion", "conclusion", "related")


def slop_lint(paper_dir: str) -> SlopReport:
    paper = Path(paper_dir)
    files: list[Path] = []
    main = paper / "main.tex"
    if main.exists():
        files.append(main)
    sections = paper / "sections"
    if sections.is_dir():
        files.extend(sorted(sections.glob("*.tex")))

    violations: list[SlopViolation] = []
    paper_occurrences: dict[str, list[tuple[str, int, str]]] = {
        token: [] for token, _, scope, _ in _THRESHOLD_BANS if scope == "paper"
    }

    for path in files:
        rel = path.relative_to(paper).as_posix()
        is_main = path.name == "main.tex"
        raw = read_text(path)
        scannable = _scannable_lines(raw, is_main)
        slug = _slug_of(path)

        for line_no, text in scannable:
            if "—" in text or " --- " in text:
                violations.append(_viol(rel, line_no, "em_dash", text))

        # Prose silently killed by a mid-line comment: content after an inline
        # `% E<n>` fact tag (or any mid-line %) never reaches the compiled PDF.
        for line_no, text in _dead_prose_lines(raw):
            violations.append(_viol(rel, line_no, "dead_prose_after_comment", text))

        for token, pattern in _HARD_BANS:
            rx = re.compile(pattern, re.IGNORECASE)
            for line_no, text in scannable:
                for _ in rx.finditer(text):
                    violations.append(_viol(rel, line_no, f"banned_phrase:{token}", text))

        for line_no, text in scannable:
            if re.search(r"\\subsubsection\b", text):
                violations.append(_viol(rel, line_no, "heading_depth", text))

        subsection_count = sum(
            len(re.findall(r"\\subsection\b", text)) for _, text in scannable
        )
        if subsection_count > 4:
            violations.append(
                SlopViolation(
                    file=rel,
                    line=0,
                    rule="heading_density",
                    excerpt=f"{subsection_count} subsection commands",
                )
            )

        if any(marker in slug for marker in _BULLET_SLUG_MARKERS):
            for line_no, text in scannable:
                if re.search(r"\\begin\{(?:itemize|enumerate)\}", text):
                    violations.append(_viol(rel, line_no, "bullet_misuse", text))

        # moreover: per-section allowance
        moreover_hits = _occurrences(scannable, r"\bmoreover\b")
        for line_no, text in moreover_hits[1:]:
            violations.append(_viol(rel, line_no, "banned_phrase:moreover", text))

        # paper-wide threshold bans: accumulate for a final pass
        for token, pattern, scope, _ in _THRESHOLD_BANS:
            if scope == "paper":
                for line_no, text in _occurrences(scannable, pattern):
                    paper_occurrences[token].append((rel, line_no, text))

        for line_no, excerpt in _not_x_but_y(scannable):
            violations.append(
                SlopViolation(file=rel, line=line_no, rule="not_x_but_y", excerpt=excerpt)
            )

        violations.extend(_rhythm_uniform(rel, scannable))
        violations.extend(_paragraph_uniform(rel, scannable))
        violations.extend(_defensive_phrase(rel, scannable))
        violations.extend(_signpost_opener(rel, scannable))
        violations.extend(_uniform_paragraph_rhythm(rel, raw))

        if is_main:
            violations.extend(_title_shape(raw, rel))

    for token, _, scope, allowance in _THRESHOLD_BANS:
        if scope == "paper":
            for rel, line_no, text in paper_occurrences[token][allowance:]:
                violations.append(_viol(rel, line_no, f"banned_phrase:{token}", text))

    # Cross-file rules that need all section paths together.
    if sections.is_dir():
        section_paths = sorted(sections.glob("*.tex"))
        violations.extend(_repeated_section_opener(paper, section_paths))

    score = max(0.0, 1.0 - 0.02 * len(violations))
    return SlopReport(violations=violations, score=score)


_DEAD_PROSE_RX = re.compile(r"\S.*?(?<!\\)%\s*(?:E\d+[\s,%E\d]*)?([^\s%].{14,})$")


def _dead_prose_lines(raw: str) -> list[tuple[int, str]]:
    """Lines where a mid-line % comment swallows ≥15 chars of substantive prose."""
    hits: list[tuple[int, str]] = []
    for line_no, text in enumerate(raw.splitlines(), start=1):
        if text.lstrip().startswith("%"):
            continue
        m = _DEAD_PROSE_RX.search(text)
        if m and re.search(r"[a-zA-Z]{3}", m.group(1)):
            hits.append((line_no, text))
    return hits


def _scannable_lines(raw: str, is_main: bool) -> list[tuple[int, str]]:
    """1-based (line_no, text) for non-comment lines; main.tex starts at \\begin{document}."""
    out: list[tuple[int, str]] = []
    started = not is_main
    for line_no, text in enumerate(raw.splitlines(), start=1):
        if not started:
            if "\\begin{document}" in text:
                started = True
            continue
        if text.lstrip().startswith("%"):
            continue
        out.append((line_no, text))
    return out


def _slug_of(path: Path) -> str:
    stem = path.stem.lower()
    return re.sub(r"^\d+[_-]?", "", stem)


def _excerpt(text: str) -> str:
    return text.strip()[:120]


def _viol(rel: str, line: int, rule: str, text: str) -> SlopViolation:
    return SlopViolation(file=rel, line=line, rule=rule, excerpt=_excerpt(text))


def _occurrences(scannable: list[tuple[int, str]], pattern: str) -> list[tuple[int, str]]:
    rx = re.compile(pattern, re.IGNORECASE)
    hits: list[tuple[int, str]] = []
    for line_no, text in scannable:
        for _ in rx.finditer(text):
            hits.append((line_no, text))
    return hits


def _delatex(text: str) -> str:
    text = re.sub(r"\$[^$]*\$", " ", text)
    text = re.sub(r"\\[a-zA-Z]+(\[[^\]]*\])?(\{[^}]*\})?", " ", text)
    return text


def _not_x_but_y(scannable: list[tuple[int, str]]) -> list[tuple[int, str]]:
    rx = re.compile(r"\bnot only\b.{0,80}\bbut also\b", re.IGNORECASE | re.DOTALL)
    results: list[tuple[int, str]] = []
    paragraph: list[tuple[int, str]] = []

    def flush() -> None:
        if not paragraph:
            return
        joined = "\n".join(text for _, text in paragraph)
        for match in rx.finditer(joined):
            results.append((_line_at(paragraph, match.start()), _excerpt(match.group(0))))

    for line_no, text in scannable:
        if text.strip() == "":
            flush()
            paragraph = []
        else:
            paragraph.append((line_no, text))
    flush()
    return results


def _line_at(paragraph: list[tuple[int, str]], offset: int) -> int:
    pos = 0
    for line_no, text in paragraph:
        if pos + len(text) >= offset:
            return line_no
        pos += len(text) + 1  # + newline
    return paragraph[0][0]


def _rhythm_uniform(rel: str, scannable: list[tuple[int, str]]) -> list[SlopViolation]:
    prose = _delatex("\n".join(text for _, text in scannable))
    sentences = [s for s in re.split(r"[.!?]", prose) if s.strip()]
    if len(sentences) < 10:
        return []
    counts = [len(s.split()) for s in sentences]
    try:
        spread = statistics.stdev(counts)
    except statistics.StatisticsError:
        spread = 0.0
    if spread < 4.0:
        return [
            SlopViolation(
                file=rel,
                line=0,
                rule="rhythm_uniform",
                excerpt=f"{len(sentences)} sentences, sentence-length stdev {spread:.2f}",
            )
        ]
    return []


def _paragraph_uniform(rel: str, scannable: list[tuple[int, str]]) -> list[SlopViolation]:
    counts: list[int] = []
    current: list[str] = []
    for _, text in scannable:
        if text.strip() == "":
            if current:
                counts.append(len(_delatex(" ".join(current)).split()))
                current = []
        else:
            current.append(text)
    if current:
        counts.append(len(_delatex(" ".join(current)).split()))
    counts = [c for c in counts if c > 0]
    if len(counts) < 6:
        return []
    mean = statistics.mean(counts)
    try:
        spread = statistics.stdev(counts)
    except statistics.StatisticsError:
        spread = 0.0
    cv = spread / mean if mean else 0.0
    if cv < 0.25:
        return [
            SlopViolation(
                file=rel,
                line=0,
                rule="paragraph_uniform",
                excerpt=f"{len(counts)} paragraphs, length cv {cv:.2f}",
            )
        ]
    return []


_DEFENSIVE_PHRASES: list[tuple[str, str]] = [
    ("unfortunately", r"\bunfortunately\b"),
    ("we were unable", r"\bwe were unable\b"),
    ("it should be noted", r"\bit should be noted\b"),
    ("we acknowledge that", r"\bwe acknowledge that\b"),
    ("we note that our method does not", r"\bwe note that our method does not\b"),
    ("we caution", r"\bwe caution\b"),
]


def _defensive_phrase(rel: str, scannable: list[tuple[int, str]]) -> list[SlopViolation]:
    """Flag defensive or apologetic constructions that belong only in Limitations."""
    out: list[SlopViolation] = []
    for label, pattern in _DEFENSIVE_PHRASES:
        rx = re.compile(pattern, re.IGNORECASE)
        for line_no, text in scannable:
            if rx.search(text):
                out.append(_viol(rel, line_no, f"defensive_phrase:{label}", text))
    return out


_SIGNPOST_PATTERNS: list[tuple[str, str]] = [
    ("in_this_section", r"\bIn this section\b"),
    ("this_section_describes", r"\bThis section describes\b"),
    ("this_section_presents", r"\bThis section presents\b"),
]


def _signpost_opener(rel: str, scannable: list[tuple[int, str]]) -> list[SlopViolation]:
    """Flag self-referential signpost sentences that carry no scientific content."""
    out: list[SlopViolation] = []
    for label, pattern in _SIGNPOST_PATTERNS:
        rx = re.compile(pattern, re.IGNORECASE)
        for line_no, text in scannable:
            if rx.search(text):
                out.append(_viol(rel, line_no, f"signpost_opener:{label}", text))
    return out


def _first_prose_sentence(raw: str) -> str:
    """Return the first prose sentence after the \\section{...} line, stripped."""
    lines = raw.splitlines()
    past_section = False
    for line in lines:
        stripped = line.strip()
        if not past_section:
            if re.search(r"\\section\*?\{", stripped):
                past_section = True
            continue
        # Skip blank lines, comments, and pure-LaTeX lines.
        if not stripped:
            continue
        if stripped.startswith("%"):
            continue
        if re.match(r"\\(?:label|begin|end|vspace|hspace|noindent|centering|small|large)\b", stripped):
            continue
        return stripped
    return ""


def _repeated_section_opener(paper: Path, section_paths: list[Path]) -> list[SlopViolation]:
    """Flag sections that open with the same first 4 words as another section."""
    openers: list[tuple[str, str, str]] = []  # (rel, first_4_words_lower, first_sentence)
    for path in section_paths:
        rel = path.relative_to(paper).as_posix()
        raw = read_text(path)
        sentence = _first_prose_sentence(raw)
        if not sentence:
            continue
        words = re.findall(r"[a-zA-Z]+", _delatex(sentence))
        if len(words) < 4:
            continue
        key = " ".join(w.lower() for w in words[:4])
        openers.append((rel, key, sentence))

    seen: dict[str, list[tuple[str, str]]] = {}
    for rel, key, sentence in openers:
        seen.setdefault(key, []).append((rel, sentence))

    out: list[SlopViolation] = []
    for key, entries in seen.items():
        if len(entries) >= 2:
            for rel, sentence in entries:
                out.append(
                    SlopViolation(
                        file=rel,
                        line=0,
                        rule="repeated_section_opener",
                        excerpt=f'first 4 words "{key}" shared with another section: {sentence[:80]}',
                    )
                )
    return out


def _uniform_paragraph_rhythm(rel: str, raw: str) -> list[SlopViolation]:
    """Flag a section file whose paragraph word counts are suspiciously uniform."""
    # Only run on section files (not main.tex).
    if not rel.startswith("sections/"):
        return []
    lines = raw.splitlines()
    paragraphs: list[list[str]] = []
    current: list[str] = []
    for line in lines:
        stripped = line.strip()
        if not stripped:
            if current:
                paragraphs.append(current)
                current = []
        else:
            # Skip pure-LaTeX structural lines.
            if re.match(r"\\(?:begin|end|section|subsection|label|vspace|hspace|centering)\b", stripped):
                continue
            current.append(stripped)
    if current:
        paragraphs.append(current)

    counts = [len(_delatex(" ".join(p)).split()) for p in paragraphs]
    counts = [c for c in counts if c > 0]
    if len(counts) < 4:
        return []
    mean = statistics.mean(counts)
    if mean == 0:
        return []
    try:
        spread = statistics.stdev(counts)
    except statistics.StatisticsError:
        spread = 0.0
    cv = spread / mean
    if cv < 0.12:
        return [
            SlopViolation(
                file=rel,
                line=0,
                rule="uniform_paragraph_rhythm",
                excerpt="paragraph lengths are uniform; vary rhythm",
            )
        ]
    return []


def _title_shape(raw: str, rel: str) -> list[SlopViolation]:
    match = re.search(r"\\title\{(.+?)\}", raw, re.DOTALL)
    if not match:
        return []
    title = match.group(1).strip()
    line_no = raw[: match.start()].count("\n") + 1
    out: list[SlopViolation] = []
    words = len(title.split())
    if words > 14:
        out.append(
            SlopViolation(file=rel, line=line_no, rule="title_shape", excerpt=f"title has {words} words")
        )
    if title.count(":") > 1:
        out.append(
            SlopViolation(file=rel, line=line_no, rule="title_shape", excerpt="title has more than one colon")
        )
    if title.endswith("?"):
        out.append(
            SlopViolation(file=rel, line=line_no, rule="title_shape", excerpt="title ends with a question mark")
        )
    return out


# --------------------------------------------------------------------------- #
# Safe .ai fallback construction                                              #
# --------------------------------------------------------------------------- #


def _is_model(annotation: object) -> bool:
    return isinstance(annotation, type) and hasattr(annotation, "model_fields")


def _neutral_value(annotation: object) -> object:
    origin = get_origin(annotation)
    if origin is None:
        if annotation is None or annotation is type(None):
            return None
        if _is_model(annotation):
            return safe_ai_fallback(annotation)
        if annotation is bool:
            return False
        if annotation is int:
            return 0
        if annotation is float:
            return 0.0
        if annotation is str:
            return ""
        return None
    if origin in (Union, getattr(types, "UnionType", Union)):
        args = get_args(annotation)
        if type(None) in args:
            return None
        non_none = [a for a in args if a is not type(None)]
        return _neutral_value(non_none[0]) if non_none else None
    if origin is list:
        return []
    if origin in (set, frozenset):
        return set() if origin is set else frozenset()
    if origin is tuple:
        return ()
    if origin is dict:
        return {}
    return None


def safe_ai_fallback(schema_cls, **overrides):
    """Build a valid instance of any model with neutral defaults; confident=False unless overridden."""
    values: dict[str, object] = {}
    for name, field in schema_cls.model_fields.items():
        if name in overrides:
            continue
        if name == "confident":
            values[name] = False
            continue
        values[name] = _neutral_value(field.annotation)
    values.update(overrides)
    return schema_cls(**values)
