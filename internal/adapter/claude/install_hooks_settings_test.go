package claude

import (
	"encoding/json"
	install "issueops/internal/adapter/install"
	"os"
	"path/filepath"
	"testing"
)

func installClaudeSettings(t *testing.T, seed *string) map[string]any {
	t.Helper()
	root := t.TempDir()
	home := t.TempDir()
	writeAdapterTestSkill(t, root, "alpha")
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if seed != nil {
		if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(settingsPath, []byte(*seed), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	req := install.DefaultNativeInstallRequest(root, home, filepath.Join(home, ".codex"), filepath.Join(root, "bin", "issueops"))
	req.SkillNames = []string{"alpha"}
	result, err := testInstaller().Install(req)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !result.OK {
		t.Fatalf("installer ok=false: %+v", result)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(readClaudeTestFile(t, settingsPath)), &settings); err != nil {
		t.Fatal(err)
	}
	return settings
}

// 비어 있거나 공백뿐인 settings.json은 없는 파일처럼 취급되어 hook이 새로 써져야 한다.
func TestClaudeInstallTreatsBlankExistingSettingsAsEmptyConfig(t *testing.T) {
	for name, content := range map[string]string{"empty": "", "whitespace": " \n\t\n"} {
		t.Run(name, func(t *testing.T) {
			settings := installClaudeSettings(t, &content)
			hooks, _ := settings["hooks"].(map[string]any)
			for _, event := range []string{"SessionStart", "SubagentStart"} {
				if groups, _ := hooks[event].([]any); len(groups) != 1 {
					t.Fatalf("%s hooks = %v", event, hooks[event])
				}
			}
		})
	}
}

// 기본 hook 그룹은 matcher 키를 갖지 않는다.
func TestClaudeInstallWritesHookGroupsWithoutMatcher(t *testing.T) {
	settings := installClaudeSettings(t, nil)
	hooks, _ := settings["hooks"].(map[string]any)
	if len(hooks) == 0 {
		t.Fatal("no hooks written")
	}
	for event, raw := range hooks {
		groups, _ := raw.([]any)
		for _, group := range groups {
			if _, has := group.(map[string]any)["matcher"]; has {
				t.Fatalf("%s group must not carry a matcher key: %v", event, group)
			}
		}
	}
}
