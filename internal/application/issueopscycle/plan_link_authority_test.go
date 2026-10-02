package issueopscycle

import (
	"context"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestPlanLinkAuthorityRequiresNativeCoordinatorInCanonicalWorkspace(t *testing.T) {
	record := model.IssueOpsRecord{Execution: &model.Execution{Mode: model.ExecutionModeOrca, Workspace: model.Workspace{Root: "/canonical"}, Lease: model.WriteLease{Status: model.LeaseStatusReleased}}}
	authority := NewMutationAuthority(func(a, b string) bool { return a == b }, liveVerifier())
	for _, host := range []string{"codex", "claude", "omo"} {
		actor := model.IssueOpsActor{Host: host, SessionID: "session", CWD: "/canonical", NativeProcessAncestry: []model.NativeProcessReceipt{{PID: 42}}}
		if err := authority.ValidatePlanLink(context.Background(), record, &actor); err != nil {
			t.Fatalf("%s coordinator refused: %v", host, err)
		}
		for _, mutate := range []func(*model.IssueOpsActor){func(a *model.IssueOpsActor) { a.Host = "unknown" }, func(a *model.IssueOpsActor) { a.SessionID = "" }, func(a *model.IssueOpsActor) { a.NativeProcessAncestry = nil }, func(a *model.IssueOpsActor) { a.CWD = "/other" }} {
			invalid := actor
			mutate(&invalid)
			if err := authority.ValidatePlanLink(context.Background(), record, &invalid); err == nil {
				t.Fatalf("invalid coordinator accepted: %+v", invalid)
			}
		}
	}
	if err := authority.ValidatePlanLink(context.Background(), record, nil); err == nil {
		t.Fatal("missing coordinator accepted")
	}
	if err := authority.ValidatePlanLink(context.Background(), model.IssueOpsRecord{}, nil); err != nil {
		t.Fatal("pre-execution linking must remain actor optional")
	}
	record.Phase = model.IssueOpsPhaseDone
	actor := model.IssueOpsActor{Host: "codex", SessionID: "session", CWD: "/canonical", NativeProcessAncestry: []model.NativeProcessReceipt{{PID: 42}}}
	if err := authority.ValidatePlanLink(context.Background(), record, &actor); err == nil {
		t.Fatal("done cycle must not use released planning exception")
	}
}

func TestPlanLinkAuthorityAcceptsCapabilityCoordinatorWithoutAncestry(t *testing.T) {
	record := model.IssueOpsRecord{Execution: &model.Execution{Mode: model.ExecutionModeOrca, Workspace: model.Workspace{Root: "/canonical"}, Lease: model.WriteLease{Status: model.LeaseStatusReleased}}}
	actor := model.IssueOpsActor{Host: "codex", SessionID: "session", CWD: "/canonical"}
	granted := verifierFunc(func(context.Context, model.NativeActor) (model.VerifiedActor, error) {
		return model.VerifiedActor{Identity: model.NativeActor{Host: "codex", SessionID: "session"}, Method: model.VerifiedByCapability}, nil
	})
	if err := NewMutationAuthority(func(a, b string) bool { return a == b }, granted).ValidatePlanLink(context.Background(), record, &actor); err != nil {
		t.Fatalf("capability coordinator refused: %v", err)
	}
	foreign := verifierFunc(func(context.Context, model.NativeActor) (model.VerifiedActor, error) {
		return model.VerifiedActor{Identity: model.NativeActor{Host: "codex", SessionID: "other"}, Method: model.VerifiedByCapability}, nil
	})
	if err := NewMutationAuthority(func(a, b string) bool { return a == b }, foreign).ValidatePlanLink(context.Background(), record, &actor); err == nil {
		t.Fatal("capability for another session linked the plan")
	}
	if err := NewMutationAuthority(func(a, b string) bool { return a == b }, liveVerifier()).ValidatePlanLink(context.Background(), record, &actor); err == nil {
		t.Fatal("native coordinator without ancestry or capability accepted")
	}
}
