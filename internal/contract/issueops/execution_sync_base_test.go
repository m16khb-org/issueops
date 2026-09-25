package issueops

import (
	"strings"
	"testing"
)

func validSyncBaseNativeActor() NativeActor {
	return NativeActor{
		Host:           "codex",
		SessionID:      "session-1",
		SessionProcess: &NativeProcessReceipt{PID: 42, StartedAt: "2026-08-25T00:00:00Z", Executable: "/usr/local/bin/codex"},
	}
}

func fullOID() string { return strings.Repeat("a", 40) }

func TestValidateNativeActorRequiresReuseSafeProcessReceipt(t *testing.T) {
	valid := validSyncBaseNativeActor()
	if err := ValidateNativeActor(valid); err != nil {
		t.Fatalf("valid actor rejected: %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(*NativeActor)
		wantErr string
	}{
		{"unknown host", func(a *NativeActor) { a.Host = "gemini" }, "host must be codex"},
		{"blank session", func(a *NativeActor) { a.SessionID = "  " }, "session_id is required"},
		{"missing process", func(a *NativeActor) { a.SessionProcess = nil }, "session_process receipt"},
		{"non positive pid", func(a *NativeActor) { a.SessionProcess.PID = 0 }, "session_process receipt"},
		{"missing started at", func(a *NativeActor) { a.SessionProcess.StartedAt = "" }, "session_process receipt"},
		{"missing executable", func(a *NativeActor) { a.SessionProcess.Executable = "" }, "session_process receipt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actor := valid
			tt.mutate(&actor)
			err := ValidateNativeActor(actor)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestBaseSyncRequiredErrorCarriesReseedFreeNextCommand(t *testing.T) {
	err := NewBaseSyncRequiredError("io-9'x", 7)

	if got := err.Error(); !strings.Contains(got, "io-9'x") || !strings.Contains(got, "completion generation 7") {
		t.Fatalf("error message mismatch: %q", got)
	}
	if !strings.Contains(err.NextCommand, "'io-9'\\''x'") || !strings.Contains(err.NextCommand, "--completion-generation 7") {
		t.Fatalf("next command quoting wrong: %q", err.NextCommand)
	}
	fields := err.IssueOpsErrorFields()
	if fields["code"] != "post_completion_sync_base_required" || fields["completion_generation"] != uint64(7) {
		t.Fatalf("fields mismatch: %+v", fields)
	}
	if fields["next_command"] != err.NextCommand {
		t.Fatal("next_command field must match error field")
	}
}
