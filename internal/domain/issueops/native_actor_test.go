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
