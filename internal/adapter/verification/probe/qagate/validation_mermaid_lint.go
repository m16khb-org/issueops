package qagate

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var mermaidUnquotedBracketTextRe = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`\[[^"\]]`) })

// LintMermaidBlocks returns style errors for every mermaid fenced block in text,
// labeled with relPath and the line number. Blocks marked with
// harness:mermaid-lint ignore are skipped.
func LintMermaidBlocks(relPath, text string) []string {
	errs := []string{}
	lines := strings.Split(text, "\n")
	var state mermaidLintState
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if state.consumeFence(trimmed) {
			continue
		}
		if !state.inMermaid || state.ignoreBlock {
			continue
		}
		errs = append(errs, lintMermaidLine(relPath, i+1, line, trimmed)...)
	}
	return errs
}

// mermaidLintState tracks which fenced block the scanner is inside.
type mermaidLintState struct {
	inMermaid         bool
	ignoreBlock       bool
	currentHeading    string
	ignoreNextMermaid bool
}

// consumeFence updates the scanner state for one trimmed line and reports
// whether the line is a code fence that must not be linted.
func (state *mermaidLintState) consumeFence(trimmed string) bool {
	if strings.Contains(trimmed, "harness:mermaid-lint ignore") {
		state.ignoreNextMermaid = true
	}
	if strings.HasPrefix(trimmed, "#") {
		state.currentHeading = trimmed
	}
	if !strings.HasPrefix(trimmed, "```") {
		return false
	}
	if !state.inMermaid {
		if strings.HasPrefix(trimmed, "```mermaid") {
			state.inMermaid = true
			state.ignoreBlock = state.ignoreNextMermaid || strings.Contains(state.currentHeading, "잘못된 예시")
			state.ignoreNextMermaid = false
		}
		return true
	}
	state.inMermaid = false
	state.ignoreBlock = false
	return true
}

// lintMermaidLine returns the mermaid style errors for one line inside a linted block.
func lintMermaidLine(relPath string, lineNo int, line, trimmed string) []string {
	var errs []string
	if strings.Contains(line, "<br>") {
		errs = append(errs, fmt.Sprintf("%s:%d mermaid uses <br>; use <br/>", relPath, lineNo))
	}
	if mermaidUnquotedBracketTextRe().MatchString(line) {
		errs = append(errs, fmt.Sprintf("%s:%d mermaid node text must start with a quote", relPath, lineNo))
	}
	if after, ok := strings.CutPrefix(trimmed, "subgraph "); ok {
		title := strings.TrimSpace(after)
		if title != "" && !strings.HasPrefix(title, `"`) {
			errs = append(errs, fmt.Sprintf("%s:%d mermaid subgraph title must be quoted", relPath, lineNo))
		}
	}
	return errs
}
