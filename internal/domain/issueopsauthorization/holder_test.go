package issueopsauthorization

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestMutationHolderRequiresCurrentLeaseAndVerifiedProcess(t *testing.T) {
	process := model.NativeProcessReceipt{PID: 1, StartedAt: "start", Executable: "/bin/codex"}
	holder := model.NativeActor{Host: "codex", SessionID: "session", AgentID: "agent", SessionProcess: &process}
	record := model.IssueOpsRecord{Execution: &model.Execution{Mode: model.ExecutionModeDirect, Workspace: model.Workspace{SourceRoot: "/repo", Root: "/repo.worktrees/run", Branch: "run", BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "then"}, Lease: model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &holder, ClaimedAt: "then"}}}
	actor := model.IssueOpsActor{Host: " CODEX ", SessionID: " session ", AgentID: " agent ", NativeProcessAncestry: []model.NativeProcessReceipt{process}}
	verified := model.VerifiedActor{Identity: holder, Method: model.VerifiedByNativeAncestry}
	if needs, err := ValidateHolder(model.IssueOpsRecord{}, nil, nil); err != nil || needs {
		t.Fatalf("unprepared planning: needs=%v err=%v", needs, err)
	}
	if needs, err := ValidateHolder(record, &actor, &verified); err != nil || !needs {
		t.Fatalf("holder: needs=%v err=%v", needs, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*model.IssueOpsActor, *model.VerifiedActor) *model.VerifiedActor
	}{
		{"foreign session", func(a *model.IssueOpsActor, v *model.VerifiedActor) *model.VerifiedActor {
			a.SessionID = "other"
			return v
		}},
		{"unverified caller", func(*model.IssueOpsActor, *model.VerifiedActor) *model.VerifiedActor { return nil }},
		{"verified other session", func(_ *model.IssueOpsActor, v *model.VerifiedActor) *model.VerifiedActor {
			v.Identity.SessionID = "other"
			return v
		}},
		{"reused pid", func(_ *model.IssueOpsActor, v *model.VerifiedActor) *model.VerifiedActor {
			v.Identity.SessionProcess = &model.NativeProcessReceipt{PID: 1, StartedAt: "other", Executable: "/bin/codex"}
			return v
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, v := actor, verified
			if _, err := ValidateHolder(record, &a, tc.change(&a, &v)); err == nil {
				t.Fatal("unauthorized holder accepted")
			}
		})
	}
	if _, err := ValidateHolder(record, nil, &verified); err == nil {
		t.Fatal("missing actor accepted")
	}
	record.Execution.Lease.Status = model.LeaseStatusReleased
	record.Execution.Lease.Holder = nil
	if _, err := ValidateHolder(record, &actor, &verified); err == nil || !strings.Contains(err.Error(), "no active write lease") {
		t.Fatalf("released error=%v", err)
	}
	if err := ValidateCWD(false); err == nil {
		t.Fatal("foreign cwd accepted")
	}
	if err := ValidateCWD(true); err != nil {
		t.Fatal(err)
	}
}
