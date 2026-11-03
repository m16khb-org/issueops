package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAdapterTestSkill(t *testing.T, root, name string) {
	t.Helper()
	path := filepath.Join(root, "skills", name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nname: "+name+"\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// jsonText is s as it appears inside a JSON string literal, so a Windows path
// can be found in a raw settings file where its backslashes are escaped.
func jsonText(t *testing.T, s string) string {
	t.Helper()
	encoded, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Trim(string(encoded), `"`)
}

// hookCommandLines renders every hook handler in a Claude settings document as
// the command line it runs, whether the handler is the shell form (command
// only) or the exec form (command plus args) Windows uses.
func hookCommandLines(t *testing.T, settings string) string {
	t.Helper()
	var document struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(settings), &document); err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, groups := range document.Hooks {
		for _, group := range groups {
			for _, handler := range group.Hooks {
				lines = append(lines, strings.Join(append([]string{handler.Command}, handler.Args...), " "))
			}
		}
	}
	return strings.Join(lines, "\n")
}
