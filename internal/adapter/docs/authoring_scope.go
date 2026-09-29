package docs

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type projectDocsManifest struct {
	Families []projectDocsFamily `json:"families"`
}
type projectDocsFamily struct {
	ModuleDir string `json:"module_dir"`
}

func (Observer) ModuleDirectories(root string) []string {
	data, err := os.ReadFile(filepath.Join(root, ".issueops", "documentation", "manifest.json"))
	if err != nil {
		return nil
	}
	var manifest projectDocsManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil
	}
	var dirs []string
	for _, family := range manifest.Families {
		dirs = append(dirs, family.ModuleDir)
	}
	return dirs
}
