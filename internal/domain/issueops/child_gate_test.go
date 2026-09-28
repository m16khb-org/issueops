package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestChildPRGateMissingPreservesValidationMatrix(t *testing.T) {
	children := []model.IssueOpsChildStatusEntry{
		{CycleID: "active", Phase: model.IssueOpsPhaseImplement},
		{CycleID: "unvalidated", Phase: model.IssueOpsPhaseDone},
		{CycleID: "rejected", Phase: model.IssueOpsPhaseDone, ValidationVerdict: "rejected"},
		{CycleID: "dropped", Phase: model.IssueOpsPhaseImplement, ValidationVerdict: "dropped", ValidationReason: "scope removed from parent", ValidatedAt: "now"},
		{CycleID: "malformed", Phase: model.IssueOpsPhaseImplement, ValidationVerdict: "dropped"},
	}
	want := []string{"child_incomplete:active", "child_unvalidated:unvalidated", "child_rejected_unresolved:rejected", "child_incomplete:malformed"}
	if got := ChildPRGateMissing(children); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing=%v want=%v", got, want)
	}
	if got := ActiveChildIDs(children); !reflect.DeepEqual(got, []string{"active", "malformed"}) {
		t.Fatalf("active=%v", got)
	}
}
