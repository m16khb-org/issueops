package codex

import "testing"

func TestWindowsHookAddsPowerShellCommandAndIsReplacedOnReinstall(t *testing.T) {
	const bin = `C:\Users\it's\issueops\bin\issueops.exe`
	windows := hooksConfigFor(bin, "windows")
	handler := windows["hooks"].(map[string]any)["SessionStart"].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)
	if got, want := handler["commandWindows"], `& 'C:\Users\it''s\issueops\bin\issueops.exe' hook session-start --host codex`; got != want {
		t.Fatalf("commandWindows = %q, want %q", got, want)
	}
	if handler["command"] == "" {
		t.Fatal("the shell-form command must stay for parsing and non-Windows hosts")
	}
	if _, present := hooksConfigFor("./bin/issueops", "")["hooks"].(map[string]any)["SessionStart"].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)["commandWindows"]; present {
		t.Fatal("the OS-neutral template must not carry commandWindows")
	}
	merged := testInstaller().mergeHookConfig(hooksConfigFor(bin, "windows"), bin)
	for event, groups := range merged["hooks"].(map[string]any) {
		if count := len(groups.([]any)); count != 1 {
			t.Fatalf("reinstall left %d %s groups: %#v", count, event, groups)
		}
	}
}
