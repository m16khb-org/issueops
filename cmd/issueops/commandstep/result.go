// Package commandstep prints self-verify step results for CLI commands that
// run verification gates.
package commandstep

import (
	selfverify "issueops/internal/contract/selfverify"

	"fmt"
	"strings"
)

// PrintStep prints a one-line outcome for step, followed by indented stdout
// and stderr when the step failed.
func PrintStep(step selfverify.StepResult) {
	if step.OK {
		fmt.Printf("→ %s ok (%dms)\n", step.Label, step.DurationMS)
		return
	}
	fmt.Printf("→ %s failed (%dms): %s\n", step.Label, step.DurationMS, step.Error)
	if step.Stdout != "" {
		fmt.Printf("  stdout:\n%s\n", IndentLines(step.Stdout))
	}
	if step.Stderr != "" {
		fmt.Printf("  stderr:\n%s\n", IndentLines(step.Stderr))
	}
}

// IndentLines splits s into lines, indents each by two spaces, and joins them
// with newlines. Blank input yields an empty string.
func IndentLines(s string) string {
	lines := splitLines(s)
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}

func splitLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}
