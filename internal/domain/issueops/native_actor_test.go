package issueops

import (
	model "issueops/internal/contract/issueops"
	"strings"
	"testing"
)

func TestValidateNativeActorRequiresReuseSafeProcessReceipt(t *testing.T) {
	valid := validSyncBaseNativeActor()
	if err := ValidateNativeActor(valid); err != nil {
		t.Fatalf("valid actor rejected: %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(*model.NativeActor)
		wantErr string
	}{
		{"unknown host", func(a *model.NativeActor) { a.Host = "gemini" }, "host must be codex"},
		{"blank session", func(a *model.NativeActor) { a.SessionID = "  " }, "session_id is required"},
		{"missing process", func(a *model.NativeActor) { a.SessionProcess = nil }, "session_process receipt"},
		{"non positive pid", func(a *model.NativeActor) { a.SessionProcess.PID = 0 }, "session_process receipt"},
		{"missing started at", func(a *model.NativeActor) { a.SessionProcess.StartedAt = "" }, "session_process receipt"},
		{"missing executable", func(a *model.NativeActor) { a.SessionProcess.Executable = "" }, "session_process receipt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actor := valid
			process := *valid.SessionProcess
			actor.SessionProcess = &process
			tt.mutate(&actor)
			err := ValidateNativeActor(actor)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestOpencodeStandaloneServerRequiresServeAndStdio(t *testing.T) {
	for _, tc := range []struct {
		name string
		args string
		want bool
	}{
		{"standalone server", "/opt/homebrew/Cellar/opencode-v2/2.0.32/bin/opencode serve --stdio --port 0", true},
		{"path with spaces", "/My Apps/opencode serve --stdio", true},
		{"shared service", "/opt/homebrew/bin/opencode serve --service", false},
		{"tui", "/opt/homebrew/bin/opencode", false},
		{"run", "/opt/homebrew/bin/opencode run --standalone hello", false},
		{"prompt mentions server", `opencode --prompt "x opencode serve --stdio"`, false},
		{"run prompt mentions server", `opencode run "opencode serve --stdio"`, false},
		{"serve without stdio", "/opt/homebrew/bin/opencode serve --port 0", false},
		{"directory named opencode", "/Apps/opencode tools/bin/opencode serve --stdio", false},
		{"other executable", "/bin/zsh serve --stdio", false},
		{"empty", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := OpencodeStandaloneServer(strings.Fields(tc.args)); got != tc.want {
				t.Fatalf("OpencodeStandaloneServer(%q) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestNormalizeNativeActorBindsOpencodeHostToOpencodeReceipt(t *testing.T) {
	server := model.NativeProcessReceipt{PID: 70, StartedAt: "2026-10-18T09:00:00Z", Executable: "/opt/homebrew/Cellar/opencode-v2/2.0.32/bin/opencode"}
	actor := model.NativeActor{
		Host: " OpenCode ", SessionID: "ses_1", SessionProcess: &server,
		ProcessAncestry: []model.NativeProcessReceipt{server, {PID: 1, StartedAt: "2026-10-01T00:00:00Z", Executable: "/sbin/launchd"}},
	}
	got, err := NormalizeNativeActor(actor)
	if err != nil || got.Host != "opencode" {
		t.Fatalf("opencode server receipt rejected: host=%q err=%v", got.Host, err)
	}
	for _, startedAt := range []string{"2026-10-18T08:59:00Z", "Sat Oct 18 08:59:00 2026"} {
		shell := model.NativeProcessReceipt{PID: 60, StartedAt: startedAt, Executable: "-zsh"}
		shellActor := model.NativeActor{
			Host: "opencode", SessionID: "ses_1", SessionProcess: &shell,
			ProcessAncestry: []model.NativeProcessReceipt{shell},
		}
		if _, err := NormalizeNativeActor(shellActor); err == nil || !strings.Contains(err.Error(), "opencode session process receipt must be the opencode standalone server") {
			t.Fatalf("shell receipt for opencode host (started_at %q): %v", startedAt, err)
		}
	}
}
