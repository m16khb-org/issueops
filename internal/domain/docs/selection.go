package docs

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	docscontract "issueops/internal/contract/docs"
)

func Roots() []string {
	return []string{"AGENTS.md", "CLAUDE.md", "GENIUS_THINK.md", ".issueops", "skills/self-verify", "skills/self-augment"}
}

// Select includes tracked docs and canonical authoring families, while always
// excluding draft and runtime evidence. Missing Git data or zero matched
// candidates preserve the standalone-directory fallback.
func Select(candidates []docscontract.Candidate, tracked map[string]bool, gitAvailable bool, moduleDirs []string) []string {
	scope := newAuthoringScope(moduleDirs)
	var eligible, matched []string
	for _, candidate := range candidates {
		if excluded(candidate.RelPath) {
			continue
		}
		eligible = append(eligible, candidate.Path)
		if candidate.RelPath != "" && (tracked[candidate.RelPath] || scope.includes(candidate.RelPath)) {
			matched = append(matched, candidate.Path)
		}
	}
	selected := eligible
	if gitAvailable && len(eligible) == 0 {
		return []string{}
	}
	if gitAvailable && len(matched) > 0 {
		selected = matched
	}
	sort.Strings(selected)
	return selected
}

// Eligible drops runtime evidence; Select only ever returns eligible paths.
func Eligible(candidates []docscontract.Candidate) []docscontract.Candidate {
	out := make([]docscontract.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !excluded(candidate.RelPath) {
			out = append(out, candidate)
		}
	}
	return out
}

func excluded(relativePath string) bool {
	const evidenceDir = ".issueops/evidence"
	return relativePath == evidenceDir || strings.HasPrefix(relativePath, evidenceDir+"/")
}

// historyDirs hold dated, per-cycle, or retired records: plans, research
// notes, issue artifacts, verification reports, and the archive. They are
// not rules an agent applies, and they are most of the index by size.
var historyDirs = []string{".issueops/issues/", ".issueops/research/", ".issueops/plans/", ".issueops/archive/", ".issueops/verified-execution/"}

// datedRecordDirs hold one dated file per decision or caution
// (YYYY-MM-DD-slug.md). The family modules in the same directories carry the
// rules and link the records, and project_docs_route picks the records a task
// touches; listing every record made the default index too large for an MCP
// result (Claude Code refused 58K characters).
var datedRecordDirs = []string{".issueops/adr/", ".issueops/cautions/"}

// HistoryRecord reports whether relativePath is a dated or retired record that
// the default docs index leaves out (callers opt in to see them).
func HistoryRecord(relativePath string) bool {
	relativePath = filepath.ToSlash(relativePath)
	for _, dir := range historyDirs {
		if strings.HasPrefix(relativePath, dir) {
			return true
		}
	}
	for _, dir := range datedRecordDirs {
		if name, ok := strings.CutPrefix(relativePath, dir); ok && !strings.Contains(name, "/") && datedName(name) {
			return true
		}
	}
	return false
}

// datedName reports whether name starts with a YYYY-MM-DD- date prefix.
func datedName(name string) bool {
	if len(name) < len("2006-01-02-") || name[10] != '-' {
		return false
	}
	_, err := time.Parse("2006-01-02", name[:10])
	return err == nil
}

type authoringScope struct{ moduleDirs []string }

func newAuthoringScope(dirs []string) authoringScope {
	scope := authoringScope{}
	for _, dir := range dirs {
		dir = filepath.ToSlash(filepath.Clean(dir))
		if strings.HasPrefix(dir, ".issueops/") && !strings.Contains(dir, "/../") && dir != ".issueops/.." {
			scope.moduleDirs = append(scope.moduleDirs, dir+"/")
		}
	}
	return scope
}

func (scope authoringScope) includes(relativePath string) bool {
	relativePath = filepath.ToSlash(filepath.Clean(relativePath))
	if filepath.ToSlash(filepath.Dir(relativePath)) == ".issueops" && filepath.Ext(relativePath) == ".md" {
		return true
	}
	if strings.HasPrefix(relativePath, ".issueops/documentation/") {
		return true
	}
	for _, dir := range scope.moduleDirs {
		if strings.HasPrefix(relativePath, dir) {
			return true
		}
	}
	return false
}
