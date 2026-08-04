package pipeline

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	DefaultModel = "openrouter/qwen/qwen3.7-flash"
	Slug         = "preprint-af"
	maxFileBytes = int64(100 * 1024 * 1024)
	inputFileCap = 400
)

var (
	skipDirs    = map[string]bool{".git": true, ".venv": true, "__pycache__": true, "node_modules": true}
	dataExts    = map[string]bool{".csv": true, ".json": true, ".parquet": true, ".npz": true, ".npy": true, ".pkl": true, ".h5": true, ".jsonl": true}
	gitIdentity = []string{"-c", "user.email=paper-improver@local", "-c", "user.name=paper-improver"}
)

func NodeID() string {
	if value := os.Getenv("AGENT_NODE_ID"); value != "" {
		return value
	}
	return Slug
}

func modelValue(model any) string {
	switch value := model.(type) {
	case string:
		return value
	case *string:
		if value != nil {
			return *value
		}
	case nil:
	default:
		return fmt.Sprint(value)
	}
	return ""
}

func AIModel(model any) string {
	if value := modelValue(model); value != "" {
		return value
	}
	if value := os.Getenv("AI_MODEL"); value != "" {
		return value
	}
	return DefaultModel
}

func OpenCodeModel(model any) string {
	if value := modelValue(model); value != "" {
		return value
	}
	if value := os.Getenv("OPENCODE_MODEL"); value != "" {
		return value
	}
	return DefaultModel
}

func FigurePython() string {
	if configured := strings.TrimSpace(os.Getenv("FIGURE_PYTHON")); configured != "" {
		return configured
	}
	if python, err := exec.LookPath("python3"); err == nil {
		return python
	}
	if python, err := exec.LookPath("python"); err == nil {
		return python
	}
	return "python3"
}

// ProjectRoot locates the repository marker from the current directory. The
// optional override is useful when the tiny binary is launched outside the repo.
func ProjectRoot() string {
	if root := os.Getenv("PREPRINT_AF_ROOT"); root != "" {
		if absolute, err := filepath.Abs(root); err == nil {
			return absolute
		}
	}
	start, err := os.Getwd()
	if err != nil {
		return "."
	}
	for current := start; ; current = filepath.Dir(current) {
		if pathExists(filepath.Join(current, "pyproject.toml")) || pathExists(filepath.Join(current, ".git")) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return start
		}
	}
}

func ReadText(path string, limit ...int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := strings.ToValidUTF8(string(data), "")
	// Go callers use 0 as the natural "no cap" sentinel.
	if len(limit) > 0 && limit[0] > 0 && len(text) > limit[0] {
		return text[:limit[0]]
	}
	return text
}

func WriteText(path, content string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func AppendTODOs(todoPath string, items []string) error {
	existing := ReadText(todoPath)
	seen := make(map[string]bool)
	for _, line := range strings.Split(existing, "\n") {
		seen[strings.TrimSpace(line)] = true
	}
	bullets := make([]string, 0, len(items))
	for _, raw := range items {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		bullet := item
		if !strings.HasPrefix(item, "- ") {
			bullet = "- " + item
		}
		if seen[bullet] || seen[item] {
			continue
		}
		seen[bullet] = true
		bullets = append(bullets, bullet)
	}
	if len(bullets) == 0 {
		return nil
	}
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	_, err := WriteText(todoPath, existing+strings.Join(bullets, "\n")+"\n")
	return err
}

func AssemblePaperText(paperDir string) string {
	parts := []string{}
	main := filepath.Join(paperDir, "main.tex")
	if fileExists(main) {
		parts = append(parts, "--- main.tex ---\n"+ReadText(main))
	}
	sections, _ := filepath.Glob(filepath.Join(paperDir, "sections", "*.tex"))
	sort.Strings(sections)
	for _, section := range sections {
		rel, err := filepath.Rel(paperDir, section)
		if err != nil {
			continue
		}
		parts = append(parts, "--- "+filepath.ToSlash(rel)+" ---\n"+ReadText(section))
	}
	return strings.Join(parts, "\n\n")
}

func SaveState(root string, state any) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	_, err = WriteText(filepath.Join(root, "STATE.json"), string(data))
	return err
}

func LoadState(root string) map[string]any {
	text := ReadText(filepath.Join(root, "STATE.json"))
	if text == "" {
		return map[string]any{}
	}
	var state map[string]any
	if err := json.Unmarshal([]byte(text), &state); err != nil || state == nil {
		return map[string]any{}
	}
	return state
}

func git(root string, args []string, timeout time.Duration) ([]byte, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, err
	}
	command := exec.Command("git", args...)
	command.Dir = root
	var timer *time.Timer
	if timeout > 0 {
		timer = time.AfterFunc(timeout, func() {
			if command.Process != nil {
				_ = command.Process.Kill()
			}
		})
		defer timer.Stop()
	}
	return command.CombinedOutput()
}

func GitSnapshot(root, label string) {
	_, _ = git(root, []string{"add", "-A"}, 60*time.Second)
	args := append(append([]string{}, gitIdentity...), "commit", "-m", label)
	_, _ = git(root, args, 60*time.Second)
}

func GitChangedFiles(root string) []string {
	output, err := git(root, []string{"status", "--porcelain"}, 30*time.Second)
	if err != nil {
		return []string{}
	}
	files := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		if len(line) <= 3 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if split := strings.SplitN(path, "->", 2); len(split) == 2 {
			path = strings.TrimSpace(split[1])
		}
		if path != "" {
			files = append(files, path)
		}
	}
	return files
}

func CreateWorkspace(folderPath string) (Workspace, error) {
	source, err := filepath.Abs(folderPath)
	if err != nil {
		return Workspace{}, err
	}
	runID, err := randomHex(6)
	if err != nil {
		return Workspace{}, err
	}
	root := filepath.Join(ProjectRoot(), ".tmp", "runs", runID)
	inputDir := filepath.Join(root, "input")
	paperDir := filepath.Join(root, "paper")
	sectionsDir := filepath.Join(paperDir, "sections")
	figuresDir := filepath.Join(paperDir, "figures")
	reviewsDir := filepath.Join(root, "reviews")
	for _, dir := range []string{inputDir, sectionsDir, figuresDir, reviewsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Workspace{}, err
		}
	}
	copyInput(source, inputDir)
	if _, err := WriteText(filepath.Join(root, "AGENTS.md"), StyleContractMarkdown()); err != nil {
		return Workspace{}, err
	}
	if _, err := WriteText(filepath.Join(root, "TODO.md"), ""); err != nil {
		return Workspace{}, err
	}
	_, _ = git(root, []string{"init"}, 30*time.Second)
	GitSnapshot(root, "initial workspace")
	return Workspace{
		RunID: runID, Root: root, InputDir: inputDir, PaperDir: paperDir,
		SectionsDir: sectionsDir, FiguresDir: figuresDir, ReviewsDir: reviewsDir,
		EvidencePath: filepath.Join(root, "EVIDENCE.md"), PositioningPath: filepath.Join(root, "POSITIONING.md"),
		BlueprintPath: filepath.Join(root, "BLUEPRINT.md"), TODOPath: filepath.Join(root, "TODO.md"),
		SourceFolder: source, InputFiles: listInputFiles(inputDir),
		HasExistingDraft: detectExistingDraft(inputDir), HasDataFiles: detectDataFiles(inputDir),
	}, nil
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func copyInput(source, inputDir string) {
	info, err := os.Stat(source)
	if err != nil || !info.IsDir() {
		return
	}
	_ = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() && path != source && skipDirs[entry.Name()] {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return nil
		}
		target := filepath.Join(inputDir, rel)
		if entry.IsDir() {
			_ = os.MkdirAll(target, 0o755)
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
			return nil
		}
		_ = copyFile(path, target, info.Mode().Perm())
		return nil
	})
}

func copyFile(source, target string, mode fs.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	return errors.Join(copyErr, closeErr)
}

func listInputFiles(inputDir string) []string {
	files := []string{}
	_ = filepath.WalkDir(inputDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(inputDir, path)
		if relErr == nil {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	if len(files) > inputFileCap {
		files = files[:inputFileCap]
	}
	return files
}

func detectExistingDraft(inputDir string) bool {
	found := false
	_ = filepath.WalkDir(inputDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || found || entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".tex" && ext != ".md" {
			return nil
		}
		text := ReadText(path)
		if len(text) <= 2000 {
			return nil
		}
		if (ext == ".md" && regexp.MustCompile(`(?m)^#{1,6}\s+\S`).MatchString(text)) ||
			(ext == ".tex" && regexp.MustCompile(`\\(sub)*section\*?\{`).MatchString(text)) {
			found = true
		}
		return nil
	})
	return found
}

func detectDataFiles(inputDir string) bool {
	found := false
	_ = filepath.WalkDir(inputDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || found || entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if dataExts[ext] {
			found = true
		} else if ext == ".py" {
			snippet := strings.ToLower(ReadText(path, 4000))
			found = strings.Contains(snippet, "matplotlib") || strings.Contains(snippet, "savefig") || strings.Contains(snippet, "plt.")
		}
		return nil
	})
	return found
}

func RunLatexmk(paperDir string, timeout ...time.Duration) (bool, string, string) {
	pdf := filepath.Join(paperDir, "main.pdf")
	if _, err := exec.LookPath("latexmk"); err != nil {
		return false, "", "latexmk not found on PATH"
	}
	limit := 420 * time.Second
	if len(timeout) > 0 {
		limit = timeout[0]
	}
	command := exec.Command("latexmk", "-pdf", "-interaction=nonstopmode", "-f", "main.tex")
	command.Dir = paperDir
	timedOut := false
	timer := time.AfterFunc(limit, func() {
		timedOut = true
		if command.Process != nil {
			_ = command.Process.Kill()
		}
	})
	err := command.Run()
	timer.Stop()
	pdfPath := ""
	if _, statErr := os.Stat(pdf); statErr == nil {
		pdfPath = pdf
	}
	if timedOut {
		excerpt := latexLogExcerpt(paperDir)
		if excerpt == "" {
			excerpt = "latexmk timed out"
		}
		return false, pdfPath, excerpt
	}
	// A freshly modified PDF is not proof of a clean build: latexmk -f may
	// emit a partial PDF while returning non-zero for missing figures or other
	// fatal errors. Only a zero exit status passes the compile gate.
	if err == nil {
		return true, pdfPath, ""
	}
	return false, pdfPath, latexLogExcerpt(paperDir)
}

func latexLogExcerpt(paperDir string) string {
	text := ReadText(filepath.Join(paperDir, "main.log"))
	if text == "" {
		return ""
	}
	matched := []string{}
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "!") || strings.Contains(line, "Error") || strings.Contains(line, "Undefined") {
			matched = append(matched, line)
		}
	}
	if len(matched) > 60 {
		matched = matched[len(matched)-60:]
	}
	excerpt := strings.Join(matched, "\n")
	if len(excerpt) > 4000 {
		excerpt = excerpt[:4000]
	}
	return excerpt
}

func StyleContractMarkdown() string {
	contract := `# AGENTS.md — paper style and fidelity contract

You are writing or editing scientific-paper artifacts inside this workspace. Follow
these rules exactly. They are read automatically for every task.

## 1. Role

- You write or edit ONE artifact of a scientific paper at a time.
- Touch ONLY the file(s) your prompt assigns. Never edit other sections, main.tex,
  the bibliography, figures, or the evidence/positioning/blueprint documents unless
  the prompt names them as your writable target.

## 2. Fidelity rules

- Every number, metric, and factual claim must trace to an EVIDENCE.md fact id. Put the
  §% E<n>§ comment on its OWN line, immediately after the sentence that uses the fact.
  NEVER append §%§ to a line that has more text after it: in LaTeX, everything after §%§
  on that line silently disappears from the compiled paper.
- Never invent results, datasets, metrics, citations, or figures.
- Missing material becomes §\todobox{what is needed and why}§; do not fabricate to fill a gap.
- Cite only keys present in §paper/refs.bib§. A needed but unknown citation becomes a §\todobox§.

## 3. Prose rules (hard bans)

- NO em dashes (—) and no § --- § in prose. Use commas, periods, or parentheses.
- Banned words and phrases: "delve", "showcase", "underscore" / "underscores", "pivotal",
  "crucial role", "paradigm", "cutting-edge", "comprehensive framework", "novel approach"
  (as a phrase), "it is worth noting", "in conclusion".
- "moreover" at most once per section. "furthermore" at most twice per paper.
  "leverage" / "leveraging" at most once per paper.
- No "not only ... but also" constructions.
- No rhetorical questions. No exclamation marks.

## 4. Prose rules (positive)

- Open each paragraph with a claim, a contrast, a mechanism, or a result.
- Vary sentence length deliberately: mix short declaratives with dense technical sentences.
- Prefer concrete verbs and domain nouns over evaluative adjectives.
- Hedge only with calibrated uncertainty tied to the evidence.
- Write transitions that carry scientific content, not signposts.

## 5. Structure rules

- No §\subsubsection§.
- Use subsections only for real method or experiment families.
- Use bullets only for genuine enumerations (contributions, assumptions, datasets).
- Never end a section with a summary of itself.

## 6. Front matter rules

- Title: at most 14 words, at most one colon, no question mark, no keyword stuffing.
- Abstract: one paragraph with the arc problem -> gap -> approach -> main quantitative
  result -> implication. No bullet-like sentence lists.

## 7. LaTeX rules

- Sections live in §paper/sections/NN_slug.tex§ and contain no §\documentclass§.
- Reference figures as §figures/<slug>.pdf§ with §\label{fig:<slug>}§.
- Captions state the takeaway, not what the axes are.
- The §\todobox{...}§ environment is defined in main.tex; do not redefine it.
- Keep the package set minimal.
`
	return strings.ReplaceAll(contract, "§", "`")
}

func escapeFrontMatter(text string) string {
	text = strings.ReplaceAll(text, "%", `\%`)
	text = strings.ReplaceAll(text, "&", `\&`)
	return strings.ReplaceAll(text, "#", `\#`)
}

func inputStem(section string) string {
	name := strings.TrimSpace(section)
	name = strings.TrimSuffix(name, ".tex")
	return strings.TrimPrefix(name, "sections/")
}

func MainTexSkeleton(title, abstract string, sectionFiles []string) string {
	inputs := make([]string, 0, len(sectionFiles))
	for _, section := range sectionFiles {
		inputs = append(inputs, `\input{sections/`+inputStem(section)+`}`)
	}
	return fmt.Sprintf(`\documentclass[11pt]{article}
\usepackage[margin=1in]{geometry}
\usepackage{amsmath}
\usepackage{amssymb}
\usepackage{graphicx}
\graphicspath{{figures/}}
\usepackage{booktabs}
\usepackage{xcolor}
\usepackage{tcolorbox}
\usepackage[numbers,sort&compress]{natbib}
\usepackage{hyperref}

\newtcolorbox{todoboxenv}{colback=yellow!8,colframe=orange!70!black,title=TODO,fonttitle=\bfseries}
\newcommand{\todobox}[1]{\begin{todoboxenv}#1\end{todoboxenv}}

\title{%s}
\author{Author Name\thanks{Draft generated for author revision.}}
\date{}

\begin{document}
\maketitle

\begin{abstract}
%s
\end{abstract}

%s

\bibliographystyle{plainnat}
\bibliography{refs}

\end{document}
`, escapeFrontMatter(title), escapeFrontMatter(abstract), strings.Join(inputs, "\n"))
}

type lintPattern struct{ token, pattern string }
type thresholdPattern struct {
	token, pattern, scope string
	allowance             int
}

var hardBans = []lintPattern{
	{"delve", `\bdelve\b`}, {"showcase", `\bshowcase(s|d|ing)?\b`},
	{"underscore", `\bunderscore(s|d)?\b`}, {"pivotal", `\bpivotal\b`},
	{"crucial role", `\bcrucial role\b`}, {"paradigm", `\bparadigm\b`},
	{"cutting-edge", `\bcutting[- ]edge\b`}, {"comprehensive framework", `\bcomprehensive framework\b`},
	{"novel approach", `\bnovel approach\b`}, {"it is worth noting", `\bit is worth noting\b`},
	{"in conclusion", `\bin conclusion\b`},
}

var thresholdBans = []thresholdPattern{
	{"moreover", `\bmoreover\b`, "file", 1}, {"furthermore", `\bfurthermore\b`, "paper", 2},
	{"leverage", `\bleverag(e|es|ed|ing)\b`, "paper", 1},
}

type numberedLine struct {
	number int
	text   string
}

func SlopLint(paperDir string) SlopReport {
	files := []string{}
	main := filepath.Join(paperDir, "main.tex")
	if fileExists(main) {
		files = append(files, main)
	}
	sections, _ := filepath.Glob(filepath.Join(paperDir, "sections", "*.tex"))
	sort.Strings(sections)
	files = append(files, sections...)
	violations := []SlopViolation{}
	paperOccurrences := map[string][]SlopViolation{}
	for _, pattern := range thresholdBans {
		if pattern.scope == "paper" {
			paperOccurrences[pattern.token] = []SlopViolation{}
		}
	}
	for _, path := range files {
		rel, _ := filepath.Rel(paperDir, path)
		rel = filepath.ToSlash(rel)
		isMain := filepath.Base(path) == "main.tex"
		raw := ReadText(path)
		lines := scannableLines(raw, isMain)
		slug := slugOf(path)
		for _, line := range lines {
			if strings.Contains(line.text, "—") || strings.Contains(line.text, " --- ") {
				violations = append(violations, violation(rel, line.number, "em_dash", line.text))
			}
		}
		for _, line := range deadProseLines(raw) {
			violations = append(violations, violation(rel, line.number, "dead_prose_after_comment", line.text))
		}
		for _, banned := range hardBans {
			rx := regexp.MustCompile(`(?i)` + banned.pattern)
			for _, line := range lines {
				for range rx.FindAllStringIndex(line.text, -1) {
					violations = append(violations, violation(rel, line.number, "banned_phrase:"+banned.token, line.text))
				}
			}
		}
		subsectionCount := 0
		headingRX := regexp.MustCompile(`\\subsubsection\b`)
		subsectionRX := regexp.MustCompile(`\\subsection\b`)
		for _, line := range lines {
			if headingRX.MatchString(line.text) {
				violations = append(violations, violation(rel, line.number, "heading_depth", line.text))
			}
			subsectionCount += len(subsectionRX.FindAllStringIndex(line.text, -1))
		}
		if subsectionCount > 4 {
			violations = append(violations, SlopViolation{File: rel, Rule: "heading_density", Excerpt: fmt.Sprintf("%d subsection commands", subsectionCount)})
		}
		if containsAny(slug, []string{"intro", "discussion", "conclusion", "related"}) {
			bulletRX := regexp.MustCompile(`\\begin\{(itemize|enumerate)\}`)
			for _, line := range lines {
				if bulletRX.MatchString(line.text) {
					violations = append(violations, violation(rel, line.number, "bullet_misuse", line.text))
				}
			}
		}
		moreover := occurrences(lines, `\bmoreover\b`)
		for _, line := range moreover[minimum(1, len(moreover)):] {
			violations = append(violations, violation(rel, line.number, "banned_phrase:moreover", line.text))
		}
		for _, pattern := range thresholdBans {
			if pattern.scope != "paper" {
				continue
			}
			for _, line := range occurrences(lines, pattern.pattern) {
				paperOccurrences[pattern.token] = append(paperOccurrences[pattern.token], violation(rel, line.number, "", line.text))
			}
		}
		for _, issue := range notXButY(lines) {
			violations = append(violations, SlopViolation{File: rel, Line: issue.number, Rule: "not_x_but_y", Excerpt: issue.text})
		}
		violations = append(violations, rhythmUniform(rel, lines)...)
		violations = append(violations, paragraphUniform(rel, lines)...)
		if isMain {
			violations = append(violations, titleShape(raw, rel)...)
		}
	}
	for _, pattern := range thresholdBans {
		if pattern.scope != "paper" {
			continue
		}
		hits := paperOccurrences[pattern.token]
		for _, hit := range hits[minimum(pattern.allowance, len(hits)):] {
			hit.Rule = "banned_phrase:" + pattern.token
			violations = append(violations, hit)
		}
	}
	return SlopReport{Violations: violations, Score: math.Max(0, 1-0.02*float64(len(violations)))}
}

func scannableLines(raw string, isMain bool) []numberedLine {
	result := []numberedLine{}
	started := !isMain
	for index, text := range strings.Split(raw, "\n") {
		if !started {
			if strings.Contains(text, `\begin{document}`) {
				started = true
			}
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(text, " \t"), "%") {
			continue
		}
		result = append(result, numberedLine{index + 1, text})
	}
	return result
}

func deadProseLines(raw string) []numberedLine {
	result := []numberedLine{}
	letters := regexp.MustCompile(`[A-Za-z]{3}`)
	for index, text := range strings.Split(raw, "\n") {
		if strings.HasPrefix(strings.TrimLeft(text, " \t"), "%") {
			continue
		}
		for pos := 1; pos < len(text); pos++ {
			if text[pos] != '%' || text[pos-1] == '\\' {
				continue
			}
			after := strings.TrimSpace(text[pos+1:])
			after = regexp.MustCompile(`^(E\d+[\s,%E\d]*)?`).ReplaceAllString(after, "")
			if len(after) >= 15 && letters.MatchString(after) {
				result = append(result, numberedLine{index + 1, text})
			}
			break
		}
	}
	return result
}

func slugOf(path string) string {
	stem := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	return regexp.MustCompile(`^\d+[_-]?`).ReplaceAllString(stem, "")
}

func excerpt(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > 120 {
		return text[:120]
	}
	return text
}

func violation(rel string, line int, rule, text string) SlopViolation {
	return SlopViolation{File: rel, Line: line, Rule: rule, Excerpt: excerpt(text)}
}

func occurrences(lines []numberedLine, pattern string) []numberedLine {
	rx := regexp.MustCompile(`(?i)` + pattern)
	result := []numberedLine{}
	for _, line := range lines {
		for range rx.FindAllStringIndex(line.text, -1) {
			result = append(result, line)
		}
	}
	return result
}

func deLatex(text string) string {
	text = regexp.MustCompile(`\$[^$]*\$`).ReplaceAllString(text, " ")
	return regexp.MustCompile(`\\[a-zA-Z]+(\[[^\]]*\])?(\{[^}]*\})?`).ReplaceAllString(text, " ")
}

func notXButY(lines []numberedLine) []numberedLine {
	rx := regexp.MustCompile(`(?is)\bnot only\b.{0,80}\bbut also\b`)
	results := []numberedLine{}
	paragraph := []numberedLine{}
	flush := func() {
		if len(paragraph) == 0 {
			return
		}
		texts := make([]string, len(paragraph))
		for i := range paragraph {
			texts[i] = paragraph[i].text
		}
		joined := strings.Join(texts, "\n")
		for _, loc := range rx.FindAllStringIndex(joined, -1) {
			results = append(results, numberedLine{lineAt(paragraph, loc[0]), excerpt(joined[loc[0]:loc[1]])})
		}
	}
	for _, line := range lines {
		if strings.TrimSpace(line.text) == "" {
			flush()
			paragraph = nil
		} else {
			paragraph = append(paragraph, line)
		}
	}
	flush()
	return results
}

func lineAt(paragraph []numberedLine, offset int) int {
	position := 0
	for _, line := range paragraph {
		if position+len(line.text) >= offset {
			return line.number
		}
		position += len(line.text) + 1
	}
	return paragraph[0].number
}

func sampleStdev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := 0.0
	for _, value := range values {
		mean += value
	}
	mean /= float64(len(values))
	sum := 0.0
	for _, value := range values {
		delta := value - mean
		sum += delta * delta
	}
	return math.Sqrt(sum / float64(len(values)-1))
}

func rhythmUniform(rel string, lines []numberedLine) []SlopViolation {
	texts := make([]string, len(lines))
	for i := range lines {
		texts[i] = lines[i].text
	}
	sentences := regexp.MustCompile(`[.!?]`).Split(deLatex(strings.Join(texts, "\n")), -1)
	counts := []float64{}
	for _, sentence := range sentences {
		if strings.TrimSpace(sentence) != "" {
			counts = append(counts, float64(len(strings.Fields(sentence))))
		}
	}
	if len(counts) < 10 {
		return nil
	}
	spread := sampleStdev(counts)
	if spread < 4 {
		return []SlopViolation{{File: rel, Rule: "rhythm_uniform", Excerpt: fmt.Sprintf("%d sentences, sentence-length stdev %.2f", len(counts), spread)}}
	}
	return nil
}

func paragraphUniform(rel string, lines []numberedLine) []SlopViolation {
	counts := []float64{}
	current := []string{}
	flush := func() {
		if len(current) > 0 {
			count := len(strings.Fields(deLatex(strings.Join(current, " "))))
			if count > 0 {
				counts = append(counts, float64(count))
			}
			current = nil
		}
	}
	for _, line := range lines {
		if strings.TrimSpace(line.text) == "" {
			flush()
		} else {
			current = append(current, line.text)
		}
	}
	flush()
	if len(counts) < 6 {
		return nil
	}
	mean := 0.0
	for _, count := range counts {
		mean += count
	}
	mean /= float64(len(counts))
	spread := sampleStdev(counts)
	cv := 0.0
	if mean != 0 {
		cv = spread / mean
	}
	if cv < 0.25 {
		return []SlopViolation{{File: rel, Rule: "paragraph_uniform", Excerpt: fmt.Sprintf("%d paragraphs, length cv %.2f", len(counts), cv)}}
	}
	return nil
}

func titleShape(raw, rel string) []SlopViolation {
	rx := regexp.MustCompile(`(?s)\\title\{(.+?)\}`)
	loc := rx.FindStringSubmatchIndex(raw)
	if loc == nil {
		return nil
	}
	title := strings.TrimSpace(raw[loc[2]:loc[3]])
	line := strings.Count(raw[:loc[0]], "\n") + 1
	out := []SlopViolation{}
	words := len(strings.Fields(title))
	if words > 14 {
		out = append(out, SlopViolation{File: rel, Line: line, Rule: "title_shape", Excerpt: fmt.Sprintf("title has %d words", words)})
	}
	if strings.Count(title, ":") > 1 {
		out = append(out, SlopViolation{File: rel, Line: line, Rule: "title_shape", Excerpt: "title has more than one colon"})
	}
	if strings.HasSuffix(title, "?") {
		out = append(out, SlopViolation{File: rel, Line: line, Rule: "title_shape", Excerpt: "title ends with a question mark"})
	}
	return out
}

// SafeAIFallback constructs a schema value with non-nil empty collections and
// zero/false neutral values. Overrides use the schema's JSON field names.
func SafeAIFallback[T any](overrides map[string]any) T {
	var value T
	initializeCollections(reflect.ValueOf(&value).Elem())
	if len(overrides) > 0 {
		base, _ := json.Marshal(value)
		merged := map[string]any{}
		_ = json.Unmarshal(base, &merged)
		for key, item := range overrides {
			merged[key] = item
		}
		if data, err := json.Marshal(merged); err == nil {
			_ = json.Unmarshal(data, &value)
		}
	}
	return value
}

func initializeCollections(value reflect.Value) {
	if !value.IsValid() || !value.CanSet() {
		return
	}
	switch value.Kind() {
	case reflect.Pointer:
		return
	case reflect.Slice:
		value.Set(reflect.MakeSlice(value.Type(), 0, 0))
	case reflect.Map:
		value.Set(reflect.MakeMap(value.Type()))
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			initializeCollections(value.Field(i))
		}
	}
}

func pathExists(path string) bool { _, err := os.Stat(path); return err == nil }
func FileExists(path string) bool { return fileExists(path) }
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
func containsAny(text string, values []string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}
func minimum(a, b int) int {
	if a < b {
		return a
	}
	return b
}
