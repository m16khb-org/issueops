package augmentation

import (
	"os"
	"path/filepath"
	"strings"
)

func (repo Repository) DocsContainTerm(root, term string) bool {
	for _, path := range repo.ListDocs(root) {
		b, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(b), term) {
			return true
		}
	}
	return false
}

func FileContainsTerm(root, relPath, term string) bool {
	b, err := os.ReadFile(filepath.Join(root, relPath))
	return err == nil && strings.Contains(string(b), term)
}

func DirContainsTerm(root, relDir, term string) bool {
	base := filepath.Join(root, relDir)
	entries, err := os.ReadDir(base)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if DirContainsTerm(root, filepath.Join(relDir, entry.Name()), term) {
				return true
			}
			continue
		}
		if filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		if FileContainsTerm(root, filepath.Join(relDir, entry.Name()), term) {
			return true
		}
	}
	return false
}
