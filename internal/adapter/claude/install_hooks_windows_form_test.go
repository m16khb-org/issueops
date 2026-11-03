package claude

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestWindowsHookUsesExecFormAndIsReplacedOnReinstall(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bin", "issueops.exe")
	installer := testInstaller()
	windows := installer.claudeSettingsConfigFor(bin, "windows")
	group := windows["hooks"].(map[string]any)["SessionStart"].([]any)[0].(map[string]any)
	handler := group["hooks"].([]any)[0].(map[string]any)
	if handler["command"] != bin || !reflect.DeepEqual(handler["args"], []any{"hook", "session-start", "--host", "claude"}) {
		t.Fatalf("windows handler = %#v", handler)
	}
	if template := installer.claudeSettingsConfigFor("./bin/issueops", ""); template["hooks"].(map[string]any)["SessionStart"].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)["args"] != nil {
		t.Fatalf("the OS-neutral template must keep the shell form: %#v", template)
	}
	for _, goos := range []string{"windows", "linux"} {
		merged := installer.mergeClaudeHookConfig(installer.claudeSettingsConfigFor(bin, "windows"), bin, goos)
		for event, groups := range merged["hooks"].(map[string]any) {
			if count := len(groups.([]any)); count != 1 {
				t.Fatalf("%s reinstall over exec-form hooks left %d %s groups: %#v", goos, count, event, groups)
			}
		}
	}
}
