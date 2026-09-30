package issueops

import (
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestChildVerdictRefusesInvalidEvidenceAndNonTerminalAcceptance(t *testing.T) {
	if _, _, err := PrepareChildVerdict("accepted", "", []string{" "}); err == nil {
		t.Fatal("accepted empty evidence")
	}
	if _, _, err := PrepareChildVerdict("dropped", "short", nil); err == nil {
		t.Fatal("dropped without substantial reason")
	}
	reason, evidence, err := PrepareChildVerdict("rejected", " missing tests ", []string{" proof ", ""})
	if err != nil || reason != "missing tests" || !reflect.DeepEqual(evidence, []string{"proof"}) {
		t.Fatalf("normalization: %q %v %v", reason, evidence, err)
	}
	child := model.IssueOpsRecord{ID: "child", Phase: model.IssueOpsPhaseProblem, Delegation: &model.IssueOpsDelegationContract{ParentCycleID: "parent"}}
	if err := ValidateChildForVerdict(child, "parent", "accepted"); err == nil || !strings.Contains(err.Error(), "child_not_done") {
		t.Fatalf("nonterminal: %v", err)
	}
	child.Phase = model.IssueOpsPhaseDone
	if err := ValidateChildForVerdict(child, "other", "accepted"); err == nil || !strings.Contains(err.Error(), "child_parent_mismatch") {
		t.Fatalf("wrong parent: %v", err)
	}
}

func TestChildVerdictPreservesParentInputAndArchivedIndexRequirement(t *testing.T) {
	parent := model.IssueOpsRecord{ID: "parent", Repo: "repo", ChildCycles: []model.IssueOpsChildCycleRef{{CycleID: "child", Title: "original", ValidationVerdict: "rejected"}}}
	child := model.IssueOpsRecord{ID: "child", Repo: "repo", Branch: "child-branch"}
	updated, ref, err := ApplyChildVerdict(parent, child, "accepted", "", []string{"checked"}, "now")
	if err != nil || ref.Title != "original" || ref.ValidationVerdict != "accepted" || ref.ValidatedAt != "now" {
		t.Fatalf("receipt: %+v %v", ref, err)
	}
	if parent.ChildCycles[0].ValidationVerdict != "rejected" {
		t.Fatal("input mutated")
	}
	if updated.ChildCycles[0].ValidationEvidence[0] != "checked" {
		t.Fatal("evidence not persisted")
	}
	if _, _, err := ApplyArchivedChildVerdict(parent, "unindexed", "accepted", "", []string{"checked"}, "now"); err == nil || !strings.Contains(err.Error(), "child_not_indexed") {
		t.Fatalf("unindexed: %v", err)
	}
	child.Repo = "other"
	if _, _, err := ApplyChildVerdict(parent, child, "accepted", "", nil, "now"); err == nil || !strings.Contains(err.Error(), "child_repo_mismatch") {
		t.Fatalf("cross repo: %v", err)
	}
}
