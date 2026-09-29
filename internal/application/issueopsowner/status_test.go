package issueopsowner

import (
	"errors"
	model "issueops/internal/contract/issueops"
	"strings"
	"testing"
)

func TestExecutionStatusProjectsRecoveryWithoutChangingRecord(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  model.LeaseStatus
		mode    model.ExecutionMode
		binding bool
		want    string
	}{
		{"direct claim", model.LeaseStatusClaimable, model.ExecutionModeDirect, false, "execution claim"},
		{"orca resume", model.LeaseStatusClaimable, model.ExecutionModeOrca, true, "execution resume"},
		{"orca incomplete", model.LeaseStatusClaimable, model.ExecutionModeOrca, false, "--preview"},
		{"released", model.LeaseStatusReleased, model.ExecutionModeDirect, false, "--preview"},
		{"revoking", model.LeaseStatusRevoking, model.ExecutionModeOrca, true, "--finalize-preview"},
		{"active", model.LeaseStatusActive, model.ExecutionModeDirect, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := model.IssueOpsRecord{ID: "io-status", Execution: &model.Execution{Mode: tc.mode, Lease: model.WriteLease{Generation: 3, Status: tc.status}}}
			if tc.binding {
				r.Execution.Orca = &model.OrcaBinding{ArtifactIdentityVersion: model.OrcaArtifactIdentityVersion, IssueBodySHA256: strings.Repeat("a", 64), ContextPacketSHA256: strings.Repeat("b", 64), OwnerPromptSHA256: strings.Repeat("c", 64)}
			}
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
