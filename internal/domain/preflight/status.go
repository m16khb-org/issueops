package preflight

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Status is the parsed `git status --short --branch` outcome: sorted,
// deduplicated file lists and whether the working tree is clean.
type Status struct {
	Staged     []string
	Unstaged   []string
	Untracked  []string
	SecretLike []string
	IsClean    bool
}

var secretPathRe = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`(?i)(^|/)(\.env(\.|$)|id_rsa|id_dsa|id_ecdsa|id_ed25519|.*\.pem$|.*\.key$|.*\.p12$|.*\.pfx$|.*credentials.*|.*secret.*)`)
})

// AnalyzeStatus parses short-format git status lines into Status. It skips the
// branch header, takes the new path of renames, and flags secret-like paths.
func AnalyzeStatus(lines []string) Status {
	var status Status
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "## ") || len(line) < 3 {
			continue
		}
		code := line[:2]
		path := strings.TrimSpace(line[3:])
		if strings.Contains(path, " -> ") {
			parts := strings.SplitN(path, " -> ", 2)
			path = parts[1]
		}
		if code == "??" {
			status.Untracked = append(status.Untracked, path)
		} else {
			if code[0] != ' ' {
				status.Staged = append(status.Staged, path)
			}
			if code[1] != ' ' {
				status.Unstaged = append(status.Unstaged, path)
			}
		}
		if secretPathRe().MatchString(filepath.ToSlash(path)) {
			status.SecretLike = append(status.SecretLike, path)
		}
	}
	status.Staged = uniqSorted(status.Staged)
	status.Unstaged = uniqSorted(status.Unstaged)
	status.Untracked = uniqSorted(status.Untracked)
	status.SecretLike = uniqSorted(status.SecretLike)
	status.IsClean = len(status.Staged) == 0 && len(status.Unstaged) == 0 && len(status.Untracked) == 0
	return status
}

// Warnings returns the preflight warning codes for a detached HEAD, a missing
// upstream, and the presence of secret-like paths. It never returns nil.
func Warnings(branch string, hasUpstream bool, secretLike []string) []string {
	warnings := []string{}
	if branch == "" {
		warnings = append(warnings, "detached_head")
	}
	if !hasUpstream {
		warnings = append(warnings, "no_upstream")
	}
	if len(secretLike) > 0 {
		warnings = append(warnings, "secret_like_paths_present")
	}
	return warnings
}

func uniqSorted(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
