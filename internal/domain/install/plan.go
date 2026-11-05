package install

import (
	"fmt"
	"path/filepath"
	"strings"
)

// CandidatePathAllowed reports whether an install candidate file may be used for target.
// Dry runs allow anything; otherwise the candidate must sit in the target's directory and be
// the target itself or a ".issueops.activate-" temporary file.
func CandidatePathAllowed(candidate, target string, dryRun bool) bool {
	if dryRun {
		return true
	}
	return filepath.Dir(candidate) == filepath.Dir(target) &&
		(candidate == target || strings.HasPrefix(filepath.Base(candidate), ".issueops.activate-"))
}

// ActivationStep trims raw and validates the native activation step: "", "begin", "seal",
// or "abort". Any non-empty step is rejected during a dry run.
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

// ValidPathMode reports whether mode is a supported PATH setup mode: auto, manual, or skip.
func ValidPathMode(mode string) bool {
	switch mode {
	case "auto", "manual", "skip":
		return true
	default:
		return false
	}
}
