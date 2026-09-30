package issueopsauthorization

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestMutationHolderRequiresCurrentLeaseAndAncestry(t *testing.T) {
	process := model.NativeProcessReceipt{PID: 1, StartedAt: "start", Executable: "/bin/codex"}
	record := model.IssueOpsRecord{Execution: &model.Execution{Mode: model.ExecutionModeDirect, Workspace: model.Workspace{SourceRoot: "/repo", Root: "/repo.worktrees/run", Branch: "run", BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "then"}, Lease: model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &model.NativeActor{Host: "codex", SessionID: "session", AgentID: "agent", SessionProcess: &process}, ClaimedAt: "then"}}}
	actor := model.IssueOpsActor{Host: " CODEX ", SessionID: " session ", AgentID: " agent ", NativeProcessAncestry: []model.NativeProcessReceipt{process}}
	if needs, err := ValidateHolder(model.IssueOpsRecord{}, nil); err != nil || needs {
		t.Fatalf("unprepared planning: needs=%v err=%v", needs, err)
	}
	if needs, err := ValidateHolder(record, &actor); err != nil || !needs {
		t.Fatalf("holder: needs=%v err=%v", needs, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*model.IssueOpsActor)
	}{
		{"foreign session", func(a *model.IssueOpsActor) { a.SessionID = "other" }},
		{"missing ancestry", func(a *model.IssueOpsActor) { a.NativeProcessAncestry = nil }},
		{"reused pid", func(a *model.IssueOpsActor) {
			a.NativeProcessAncestry = []model.NativeProcessReceipt{{PID: 1, StartedAt: "other", Executable: "/bin/codex"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := actor
			tc.change(&a)
			if _, err := ValidateHolder(record, &a); err == nil {
				t.Fatal("unauthorized holder accepted")
			}
		})
	}
	if _, err := ValidateHolder(record, nil); err == nil {
		t.Fatal("missing actor accepted")
	}
	record.Execution.Lease.Status = model.LeaseStatusReleased
	record.Execution.Lease.Holder = nil
	if _, err := ValidateHolder(record, &actor); err == nil || !strings.Contains(err.Error(), "no active write lease") {
		t.Fatalf("released error=%v", err)
	}
	if err := ValidateCWD(false); err == nil {
		t.Fatal("foreign cwd accepted")
	}
	if err := ValidateCWD(true); err != nil {
		t.Fatal(err)
	}
}
