package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestExecutionReadinessMissingPreservesValidationPathAndLeaseOrder(t *testing.T) {
	if got := ExecutionReadinessMissing(model.IssueOpsRecord{}, false); !reflect.DeepEqual(got, []string{"execution"}) {
		t.Fatalf("missing execution=%v", got)
	}
	record := model.IssueOpsRecord{Execution: &model.Execution{}}
	if got := ExecutionReadinessMissing(record, false); !reflect.DeepEqual(got, []string{"execution_valid", "execution_worktree_match", "execution_write_lease"}) {
		t.Fatalf("invalid execution=%v", got)
	}
	if got := ExecutionReadinessMissing(record, true); !reflect.DeepEqual(got, []string{"execution_valid", "execution_write_lease"}) {
		t.Fatalf("matching workspace=%v", got)
	}
}
