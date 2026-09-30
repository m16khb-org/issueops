package install

import (
	"os"
	"path/filepath"

	installdomain "issueops/internal/domain/install"
)

func ListSkillNames(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && exists(filepath.Join(root, "skills", entry.Name(), "SKILL.md")) {
			names = append(names, entry.Name())
		}
	}
	return normalizeSkillNames(names), nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func normalizeSkillNames(names []string) []string {
	return installdomain.NormalizeSkillNames(names)
}
