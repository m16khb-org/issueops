package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestLinkedBranchAuditRequiresTheObservedCycleAndTarget(t *testing.T) {
	observed := model.IssueOpsRecord{
		ID: "cycle", Repo: "/repo", IssueURL: "issue", Branch: "topic", CreatedAt: "created",
		BranchPrepare: &model.IssueOpsBranchPrepare{Provider: "github", IssueURL: "issue", Branch: "topic", BaseSHA: "base", BaseBranch: "main", CreatedAt: "prepared"},
	}
	for _, field := range []string{"id", "repo", "issue", "branch", "created", "missing prepare", "provider", "prepared issue", "prepared branch", "base SHA", "base branch", "prepared time"} {
		t.Run(field, func(t *testing.T) {
			current := observed
			prepare := *observed.BranchPrepare
			current.BranchPrepare = &prepare
			switch field {
			case "id":
				current.ID = "replacement"
			case "repo":
				current.Repo = "/other"
			case "issue":
				current.IssueURL = "other"
			case "branch":
				current.Branch = "other"
			case "created":
				current.CreatedAt = "recreated"
			case "missing prepare":
				current.BranchPrepare = nil
			case "provider":
				prepare.Provider = "gitlab"
			case "prepared issue":
				prepare.IssueURL = "other"
			case "prepared branch":
				prepare.Branch = "other"
			case "base SHA":
				prepare.BaseSHA = "new-base"
			case "base branch":
				prepare.BaseBranch = "release"
			case "prepared time":
				prepare.CreatedAt = "reprepared"
			}
			got, err := ApplyLinkedBranchCleanupAudit(observed, current, model.CleanupLinkedBranchResult{AlreadyAbsent: true}, "now")
			if err == nil || !reflect.DeepEqual(got, current) {
				t.Fatalf("changed target accepted or modified: got=%+v err=%v", got, err)
			}
		})
	}
}

func TestLinkedBranchAuditProjectsReceiptWithoutMutatingCurrentMetadata(t *testing.T) {
	prior := &model.IssueOpsLinkedBranchCleanup{State: "absent", ObservedAt: "old"}
	observed := model.IssueOpsRecord{ID: "cycle", BranchPrepare: &model.IssueOpsBranchPrepare{Branch: "topic"}, LinkedBranchCleanup: prior}
	current := observed
	current.PlanPath, current.UpdatedAt = "new-plan.md", "updated"
	result := model.CleanupLinkedBranchResult{State: "orphan", StateReason: "reason", LinkedBranchID: "node", LinkedCount: 1, RemoteRefOID: "oid", Fingerprint: "fingerprint", Deleted: true, FailedStep: "step"}
	got, err := ApplyLinkedBranchCleanupAudit(observed, current, result, "now")
	want := current
	want.LinkedBranchCleanup = &model.IssueOpsLinkedBranchCleanup{State: "orphan", StateReason: "reason", LinkedBranchID: "node", LinkedCount: 1, RemoteRefOID: "oid", Fingerprint: "fingerprint", Deleted: true, FailedStep: "step", ObservedAt: "now"}
	if err != nil || !reflect.DeepEqual(got, want) || prior.State != "absent" || prior.ObservedAt != "old" {
		t.Fatalf("receipt or input changed: got=%+v prior=%+v err=%v", got, prior, err)
	}
}
