package guard

import (
	guarddomain "issueops/internal/domain/guard"
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/domain/guardpattern"
)

func guardExistingSymbols(root string, targetFiles []string) map[string][]string {
	targets := stringSet(targetFiles...)
	symbols := map[string][]string{}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "bin" || name == ".cache" || name == ".codex" || name == ".codegraph" || name == ".omx" || name == ".omc" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if targets[rel] || !guarddomain.SourcePath(rel) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, line := range strings.Split(string(b), "\n") {
			if m := pattern.NewSymbol.FindStringSubmatch(line); len(m) == 2 {
				key := guarddomain.NormalizeSymbol(m[1])
				if key != "" {
					symbols[key] = append(symbols[key], rel)
				}
			}
		}
		return nil
	})
	for key := range symbols {
		symbols[key] = uniqSorted(symbols[key])
	}
	return symbols
}
