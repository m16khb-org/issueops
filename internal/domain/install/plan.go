package install

import (
	"fmt"
	"path/filepath"
	"strings"
)

func CandidatePathAllowed(candidate, target string, dryRun bool) bool {
	if dryRun {
		return true
	}
	return filepath.Dir(candidate) == filepath.Dir(target) &&
		(candidate == target || strings.HasPrefix(filepath.Base(candidate), ".issueops.activate-"))
}

func ActivationStep(dryRun bool, raw string) (string, error) {
	step := strings.TrimSpace(raw)
	if dryRun && step != "" {
		return "", fmt.Errorf("native activation step is not valid during dry-run")
	}
	switch step {
	case "", "begin", "seal", "abort":
		return step, nil
	default:
		return "", fmt.Errorf("invalid native activation step %q", step)
	}
}

func ValidPathMode(mode string) bool {
	switch mode {
	case "auto", "manual", "skip":
		return true
	default:
		return false
	}
}
