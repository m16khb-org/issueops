package docs

import (
	docscontract "issueops/internal/contract/docs"
	"path/filepath"
	"sort"
	"strings"
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
