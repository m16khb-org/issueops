package issueopscycle

import (
	"errors"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestNativeActorNormalizationOrdersValidationAndLiveInspection(t *testing.T) {
	process := model.NativeProcessReceipt{PID: 42, StartedAt: "started", Executable: "/bin/codex"}
	input := model.NativeActor{Host: " CODEX ", SessionID: " session ", AgentID: " agent ", SessionProcess: &model.NativeProcessReceipt{PID: 42, StartedAt: " started ", Executable: " /bin/codex "}, ProcessAncestry: []model.NativeProcessReceipt{process}}
	for _, mode := range []string{"live", "host", "ancestry", "inspect-error", "dead", "reused"} {
		t.Run(mode, func(t *testing.T) {
			actor := input
			if mode == "host" {
				actor.Host = "other"
			}
			if mode == "ancestry" {
				actor.ProcessAncestry = nil
			}
			calls := 0
			got, err := NormalizeNativeActor(actor, func(receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
				calls++
				if receipt != process {
					t.Fatalf("receipt=%+v", receipt)
				}
				if mode == "inspect-error" {
					return "", model.NativeProcessReceipt{}, errors.New("inspection failed")
				}
				if mode == "dead" {
					return "dead", model.NativeProcessReceipt{}, nil
				}
				observed := process
				if mode == "reused" {
					observed.StartedAt = "different"
				}
				return "live", observed, nil
			})
			if mode == "live" {
				if err != nil || got.Host != "codex" || got.SessionID != "session" || got.AgentID != "agent" || *got.SessionProcess != process {
					t.Fatalf("actor=%+v err=%v", got, err)
				}
				got.SessionProcess.Executable = "changed"
				got.ProcessAncestry[0].Executable = "changed"
				if input.SessionProcess.Executable != " /bin/codex " || input.ProcessAncestry[0] != process {
					t.Fatal("normalization mutated caller identity")
				}
			} else {
				want := map[string]string{"host": "host must", "ancestry": "local process ancestry", "inspect-error": "inspection failed", "dead": "not live", "reused": "does not match live PID"}[mode]
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("err=%v want=%s", err, want)
				}
			}
			wantCalls := 1
			if mode == "host" || mode == "ancestry" {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Fatalf("inspection calls=%d want=%d", calls, wantCalls)
			}
		})
	}
}
