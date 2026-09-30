package issueops

import (
	model "issueops/internal/contract/issueops"
	"reflect"
	"testing"
)

func TestPRGateAdmissionAndReadinessPreserveContract(t *testing.T) {
	if !NeedsPRGateRead(" pr ") || NeedsPRGateRead("feedback") || NeedsPRGateEvaluation(model.IssueOpsPhasePR) || !NeedsPRGateEvaluation(model.IssueOpsPhaseFeedback) {
		t.Fatal("wrong PR gate admission")
	}
	ready := model.IssueOpsReadiness{Ready: true, Missing: []string{"base"}, Warnings: []string{"existing"}}
	if got := MergeGateReadiness(ready, nil, nil); !reflect.DeepEqual(got, ready) {
		t.Fatalf("unchanged=%+v", got)
	}
	got := MergeGateReadiness(ready, []string{"loop_active", "base"}, []string{"loop running"})
	if got.Ready || !reflect.DeepEqual(got.Missing, []string{"base", "loop_active"}) || !reflect.DeepEqual(got.Warnings, []string{"existing", "loop running"}) {
		t.Fatalf("merged=%+v", got)
	}
	if err := PRGateError(got); err == nil || err.Error() != "cannot enter pr phase: missing base, loop_active" {
		t.Fatalf("err=%v", err)
	}
	if err := PRGateError(model.IssueOpsReadiness{Ready: true}); err != nil {
		t.Fatal(err)
	}
}
