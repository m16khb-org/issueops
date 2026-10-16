package issueopsowner

import (
	"errors"
	model "issueops/internal/contract/issueops"
	"strings"
	"testing"
)

func TestExecutionStatusProjectsRecoveryWithoutChangingRecord(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status model.LeaseStatus
		want   string
	}{
		{"claim", model.LeaseStatusClaimable, "execution claim"},
		{"released", model.LeaseStatusReleased, "--preview"},
		{"revoking", model.LeaseStatusRevoking, "--finalize-preview"},
		{"active", model.LeaseStatusActive, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := model.IssueOpsRecord{ID: "io-status", Execution: &model.Execution{Lease: model.WriteLease{Generation: 3, Status: tc.status}}}
			reads := 0
			s := ExecutionStatus{ReadRecord: func(id string) (model.IssueOpsRecord, error) {
				reads++
				if id != r.ID {
					t.Fatal(id)
				}
				return r, nil
			}}
			result, err := s.Read(r.ID)
			if err != nil || !result.OK || reads != 1 || (tc.want == "" && result.NextCommand != "") || (tc.want != "" && !strings.Contains(result.NextCommand, tc.want)) {
				t.Fatalf("status: %+v %v reads=%d", result, err, reads)
			}
			if r.Execution.Lease.Generation != 3 || r.Execution.Lease.Status != tc.status {
				t.Fatal("read-only status changed lease")
			}
			r.Execution.Completion = &model.ExecutionCompletion{}
			result, err = s.Read(r.ID)
			if err != nil || result.NextCommand != "" {
				t.Fatalf("completed execution offers recovery: %+v %v", result, err)
			}
		})
	}
}
func TestExecutionStatusReadErrorsAndUnpreparedExecution(t *testing.T) {
	missing := errors.New("missing record")
	for _, failure := range []error{missing, nil} {
		s := ExecutionStatus{ReadRecord: func(string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, failure }}
		result, err := s.Read("missing")
		if err == nil || result.OK || result.ID != "missing" || (failure != nil && !errors.Is(err, failure)) {
			t.Fatalf("status error contract: %+v %v", result, err)
		}
	}
}

func TestWriterlessCommandIsEmptyWithoutExecution(t *testing.T) {
	if got := WriterlessCommand(model.IssueOpsRecord{ID: "io-1"}); got != "" {
		t.Fatalf("got %q", got)
	}
}
