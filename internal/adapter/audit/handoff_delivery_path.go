package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// handoffDeliveryStateRootPath canonicalizes only a platform-defined trusted
// prefix. Components below that prefix remain unresolved and are opened one by
// one without following links.
func handoffDeliveryStateRootPath(stateRoot string) (string, error) {
	stateRoot = filepath.Clean(strings.TrimSpace(stateRoot))
	if !filepath.IsAbs(stateRoot) {
		return "", fmt.Errorf("handoff delivery state root must be absolute")
	}
	for _, base := range handoffDeliveryTrustedBases() {
		base = filepath.Clean(base)
		rel, err := filepath.Rel(base, stateRoot)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			continue
		}
		canonicalBase, err := filepath.EvalSymlinks(base)
		if err != nil {
			return "", fmt.Errorf("resolve trusted handoff delivery state base: %w", err)
		}
		return filepath.Join(canonicalBase, rel), nil
	}
	return stateRoot, nil
}

func validHandoffDeliveryWindowsMode(mode os.FileMode, directory bool) bool {
	if mode&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return false
	}
	if directory {
		return mode.IsDir()
	}
	return mode.IsRegular()
}
