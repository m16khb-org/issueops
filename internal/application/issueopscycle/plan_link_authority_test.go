package issueopscycle

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestPlanLinkAuthorityRequiresNativeCoordinatorInCanonicalWorkspace(t *testing.T) {
	record := model.IssueOpsRecord{Execution: &model.Execution{Mode: model.ExecutionModeOrca, Workspace: model.Workspace{Root: "/canonical"}, Lease: model.WriteLease{Status: model.LeaseStatusReleased}}}
	authority := NewMutationAuthority(func(a, b string) bool { return a == b })
	for _, host := range []string{"codex", "claude", "omo"} {
		actor := model.IssueOpsActor{Host: host, SessionID: "session", CWD: "/canonical", NativeProcessAncestry: []model.NativeProcessReceipt{{PID: 42}}}
		if err := authority.ValidatePlanLink(record, &actor); err != nil {
			t.Fatalf("%s coordinator refused: %v", host, err)
		}
		for _, mutate := range []func(*model.IssueOpsActor){func(a *model.IssueOpsActor) { a.Host = "unknown" }, func(a *model.IssueOpsActor) { a.SessionID = "" }, func(a *model.IssueOpsActor) { a.NativeProcessAncestry = nil }, func(a *model.IssueOpsActor) { a.CWD = "/other" }} {
			invalid := actor
			mutate(&invalid)
			if err := authority.ValidatePlanLink(record, &invalid); err == nil {
				t.Fatalf("invalid coordinator accepted: %+v", invalid)
			}
		}
	}
	if err := authority.ValidatePlanLink(record, nil); err == nil {
		t.Fatal("missing coordinator accepted")
	}
	if err := authority.ValidatePlanLink(model.IssueOpsRecord{}, nil); err != nil {
		t.Fatal("pre-execution linking must remain actor optional")
	}
	record.Phase = model.IssueOpsPhaseDone
	actor := model.IssueOpsActor{Host: "codex", SessionID: "session", CWD: "/canonical", NativeProcessAncestry: []model.NativeProcessReceipt{{PID: 42}}}
	if err := authority.ValidatePlanLink(record, &actor); err == nil {
		t.Fatal("done cycle must not use released planning exception")
	}
}
