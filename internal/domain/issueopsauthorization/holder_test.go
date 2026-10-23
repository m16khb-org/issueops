package issueopsauthorization

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestMutationHolderRequiresCurrentLeaseAndVerifiedProcess(t *testing.T) {
	process := model.NativeProcessReceipt{PID: 1, StartedAt: "start", Executable: "/bin/codex"}
	holder := model.NativeActor{Host: "codex", SessionID: "session", AgentID: "agent", SessionProcess: &process}
	record := model.IssueOpsRecord{Execution: &model.Execution{Workspace: model.Workspace{SourceRoot: "/repo", Root: "/repo.worktrees/run", Branch: "run", BaseHead: strings.Repeat("a", 40), LinkedAt: "then"}, Lease: model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &holder, ClaimedAt: "then"}}}
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
		detail string
	}{
		{"no actor flags", func(a *model.IssueOpsActor, v *model.VerifiedActor) *model.VerifiedActor {
			*a = model.IssueOpsActor{}
			return v
		}, "no actor flags were passed"},
		{"foreign host", func(a *model.IssueOpsActor, v *model.VerifiedActor) *model.VerifiedActor {
			a.Host = "claude"
			return v
		}, `caller host "claude", holder host "codex"`},
		{"foreign session", func(a *model.IssueOpsActor, v *model.VerifiedActor) *model.VerifiedActor {
			a.SessionID = "other"
			return v
		}, `caller session id "other", holder session id "session"`},
		{"foreign agent", func(a *model.IssueOpsActor, v *model.VerifiedActor) *model.VerifiedActor {
			a.AgentID = ""
			return v
		}, `caller agent id "", holder agent id "agent"`},
		{"unverified caller", func(*model.IssueOpsActor, *model.VerifiedActor) *model.VerifiedActor { return nil }, "verified session process"},
		{"verified other session", func(_ *model.IssueOpsActor, v *model.VerifiedActor) *model.VerifiedActor {
			v.Identity.SessionID = "other"
			return v
		}, "verified session process"},
		{"reused pid", func(_ *model.IssueOpsActor, v *model.VerifiedActor) *model.VerifiedActor {
			v.Identity.SessionProcess = &model.NativeProcessReceipt{PID: 1, StartedAt: "other", Executable: "/bin/codex"}
			return v
		}, "verified session process"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, v := actor, verified
			_, err := ValidateHolder(record, &a, tc.change(&a, &v))
			// The refusal names the mismatch and the recovery command.
			if err == nil || !strings.Contains(err.Error(), tc.detail) || !strings.Contains(err.Error(), "execution whoami --json") {
				t.Fatalf("err=%v, want %q and the whoami recovery step", err, tc.detail)
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
