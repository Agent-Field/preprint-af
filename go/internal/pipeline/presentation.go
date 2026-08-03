package pipeline

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	cleanPaperBegin = "% preprint-af clean-paper begin"
	cleanPaperEnd   = "% preprint-af clean-paper end"
)

var cleanPaperBlockPattern = regexp.MustCompile(`(?s)\n?% preprint-af clean-paper begin.*?% preprint-af clean-paper end\n?`)

// ApplyPaperPresentationPolicy changes only PDF presentation. Evidence gaps
// remain in TODO.md and REVIEW.md, but clean papers do not print draft TODO
// callouts in the manuscript unless the caller explicitly opts in.
func ApplyPaperPresentationPolicy(ws Workspace, showTODOs bool) error {
	mainPath := filepath.Join(ws.PaperDir, "main.tex")
	body := ReadText(mainPath, 0)
	if body == "" {
		return fmt.Errorf("paper/main.tex is missing")
	}
	body = cleanPaperBlockPattern.ReplaceAllString(body, "\n")
	if !showTODOs {
		block := cleanPaperBegin + "\n" +
			"\\renewcommand{\\todobox}[1]{}\n" +
			cleanPaperEnd + "\n"
		marker := "\\begin{document}"
		if !strings.Contains(body, marker) {
			return fmt.Errorf("paper/main.tex has no \\begin{document}")
		}
		body = strings.Replace(body, marker, block+marker, 1)
	}
	_, err := WriteText(mainPath, body)
	return err
}
