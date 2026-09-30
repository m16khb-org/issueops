package issueopscycle

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestMutationAuthorityValidatesExecutionBeforePathObservation(t *testing.T) {
	calls := 0
	authority := NewMutationAuthority(func(string, string) bool { calls++; return true })
	if err := authority.Validate(model.IssueOpsRecord{}, nil); err != nil || calls != 0 {
		t.Fatalf("planning err=%v paths=%d", err, calls)
	}
	record := model.IssueOpsRecord{Execution: &model.Execution{}}
	if err := authority.Validate(record, nil); err == nil || !strings.Contains(err.Error(), "invalid IssueOps execution v1 record") || calls != 0 {
		t.Fatalf("invalid execution err=%v paths=%d", err, calls)
	}
}

func TestAuthorizeHolderObservesPathsOnlyAfterIdentityMatches(t *testing.T) {
	process := model.NativeProcessReceipt{PID: 42, StartedAt: "start", Executable: "/bin/host"}
	record := model.IssueOpsRecord{Execution: &model.Execution{Workspace: model.Workspace{Root: "/canonical"}, Lease: model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &model.NativeActor{Host: "codex", SessionID: "holder", SessionProcess: &process}}}}
	actor := model.IssueOpsActor{Host: "codex", SessionID: "holder", CWD: "/request", NativeProcessAncestry: []model.NativeProcessReceipt{process}}
	calls := 0
	paths := func(left, right string) bool {
		calls++
		if left != "/request" || right != "/canonical" {
			t.Fatalf("paths=%q %q", left, right)
		}
		return false
	}
	if err := AuthorizeHolder(record, &actor, paths); err == nil || calls != 1 {
		t.Fatalf("foreign path error=%v calls=%d", err, calls)
	}
	actor.SessionID = "other"
	if err := AuthorizeHolder(record, &actor, paths); err == nil || calls != 1 {
		t.Fatalf("foreign holder error=%v calls=%d", err, calls)
	}
	actor.SessionID = "holder"
	if err := AuthorizeHolder(record, &actor, nil); err == nil {
		t.Fatal("missing path observer accepted")
	}
}
