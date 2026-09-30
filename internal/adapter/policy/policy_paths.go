package policy

import (
	"os"
	"path/filepath"
	"strings"

	policydomain "issueops/internal/domain/policy"
)

func absOrOriginal(path string) string {
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func canonicalPotentialPath(path string) string {
	if path == "" {
		return ""
	}
	abs := absOrOriginal(path)
	if eval, err := filepath.EvalSymlinks(abs); err == nil {
		return eval
	}
	originalAbs := abs
	missing := []string{}
	for {
		parent := filepath.Dir(abs)
		if parent == abs {
			return originalAbs
		}
		missing = append([]string{filepath.Base(abs)}, missing...)
		if eval, err := filepath.EvalSymlinks(parent); err == nil {
			parts := append([]string{eval}, missing...)
			return filepath.Join(parts...)
		}
		abs = parent
	}
}

func sameOrWithin(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func commandReferencesOutsideWorkspace(root, cwd string, argv []string) bool {
	if root == "" || cwd == "" || len(argv) < 2 {
		return false
	}
	for _, arg := range argv[1:] {
		for _, candidate := range policydomain.PathCandidates(arg) {
			resolved := resolvePolicyPathCandidate(cwd, candidate)
			if resolved == "" {
				continue
			}
			if !sameOrWithin(root, canonicalPotentialPath(resolved)) {
				return true
			}
		}
	}
	return false
}

func resolvePolicyPathCandidate(cwd, candidate string) string {
	if strings.TrimSpace(candidate) == "" || strings.HasPrefix(candidate, "~") {
		if candidate == "~" || strings.HasPrefix(candidate, "~/") || strings.HasPrefix(candidate, "~"+string(os.PathSeparator)) {
			home, err := os.UserHomeDir()
			if err != nil || home == "" {
				return candidate
			}
			rest := strings.TrimPrefix(strings.TrimPrefix(candidate, "~/"), "~"+string(os.PathSeparator))
			if rest == candidate {
				rest = ""
			}
			return filepath.Join(home, rest)
		}
		return candidate
	}
	if filepath.IsAbs(candidate) {
		return candidate
	}
	return filepath.Join(cwd, candidate)
}
