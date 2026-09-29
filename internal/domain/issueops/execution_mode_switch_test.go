package issueops

import (
	model "issueops/internal/contract/issueops"
	"reflect"
	"testing"
)

func TestResetExecutionPreservesCycleAndDoesNotMutateInput(t *testing.T) {
	record := model.IssueOpsRecord{ID: "cycle", Repo: "repo", Branch: "branch", PlanPath: "plan", WorktreePath: "worktree", Execution: &model.Execution{Mode: model.ExecutionModeDirect, Lease: model.WriteLease{Generation: 7}}, UpdatedAt: "original"}
	reset := ResetExecutionForModeSwitch(record)
	if record.Execution == nil || record.PlanPath != "plan" || record.WorktreePath != "worktree" {
		t.Fatal("reset mutated input")
	}
	want := record
	want.Execution = nil
	want.PlanPath = ""
	want.WorktreePath = ""
	if !reflect.DeepEqual(reset, want) {
		t.Fatalf("reset=%+v want=%+v", reset, want)
	}
}
