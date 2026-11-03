package codex

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
// can be found in a raw JSON file where its backslashes are escaped.
func jsonText(t *testing.T, s string) string {
	t.Helper()
	encoded, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Trim(string(encoded), `"`)
}

// hookShellCommands returns the shell-form "command" of every hook handler in
// a hooks.json document. Windows installs also add a commandWindows line that
// repeats the same invocation, so counting raw text would double count.
func hookShellCommands(t *testing.T, raw string) []string {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(raw), &document); err != nil {
		t.Fatal(err)
	}
	return shellCommandsIn(document["hooks"])
}

// shellCommandsIn collects the "command" of every handler under a hooks value
// (an event map, or the groups of one event).
func shellCommandsIn(hooks any) []string {
	var commands []string
	var walk func(value any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			if command, ok := typed["command"].(string); ok {
				commands = append(commands, command)
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(hooks)
	return commands
}

func countContaining(commands []string, needle string) int {
	count := 0
	for _, command := range commands {
		if strings.Contains(command, needle) {
			count++
		}
	}
	return count
}
