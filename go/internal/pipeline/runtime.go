package pipeline

import (
	"fmt"
	"os/exec"
	"strings"
)

// runtimePreflight catches non-repairable host setup failures before the
// workflow spends tokens. The container image supplies all of these tools;
// native af installs intentionally keep the Go artifact small and use host
// executables.
func runtimePreflight() error {
	required := []string{"opencode", "latexmk", "pdflatex"}
	missing := make([]string, 0, len(required))
	for _, name := range required {
		if _, err := exec.LookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("runtime preflight failed: missing %s on PATH; use the Docker image for a self-contained runtime or install the host prerequisites", strings.Join(missing, ", "))
	}
	python := FigurePython()
	if resolved, err := exec.LookPath(python); err == nil {
		probe := exec.Command(resolved, "-c", "import matplotlib, numpy, pandas, sklearn")
		if err := probe.Run(); err != nil {
			fmt.Println("[preflight] warning: figure Python lacks matplotlib/numpy/pandas/scikit-learn; generated figure scripts may fail (the Docker image includes these packages)")
		}
	} else {
		fmt.Println("[preflight] warning: python3 not found; existing figures still work, but Python figure scripts cannot run (the Docker image includes Python)")
	}
	return nil
}
