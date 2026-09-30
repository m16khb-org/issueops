package issueopsowner

import (
	"fmt"
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	leasedomain "issueops/internal/domain/issueopslease"
)

type ExecutionStatus struct {
	ReadRecord func(string) (issueops.IssueOpsRecord, error)
}

func (s ExecutionStatus) Read(id string) (issueops.ExecutionResult, error) {
	record, err := s.ReadRecord(id)
	if err != nil {
		return issueops.ExecutionResult{OK: false, ID: id}, err
	}
	if record.Execution == nil {
		return issueops.ExecutionResult{OK: false, ID: id}, fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	result := issueops.ExecutionResult{OK: true, ID: record.ID, Execution: *record.Execution}
	if record.Execution.Completion == nil {
		result.NextCommand = WriterlessCommand(record)
	}
	return result, nil
}
func WriterlessCommand(record issueops.IssueOpsRecord) string {
	if record.Execution == nil {
		return ""
	}
	lease := record.Execution.Lease
	identityComplete := false
	if lease.Status == issueops.LeaseStatusClaimable && record.Execution.Mode == issueops.ExecutionModeOrca {
		identityComplete = domain.CompleteOwnerArtifactIdentity(record.Execution.Orca)
	}
	switch leasedomain.DecideWriterlessRecovery(string(lease.Status), string(record.Execution.Mode), identityComplete) {
	case leasedomain.RecoveryReplacePreview:
		return domain.ReplacementPreviewCommand(record.ID, lease.Generation)
	case leasedomain.RecoveryResume:
		return domain.ReplacementResumeCommand(record.ID, lease.Generation)
	case leasedomain.RecoveryDirectClaim:
		return domain.ReplacementClaimCommand(record.ID, lease.Generation, "")
	case leasedomain.RecoveryFinalizePreview:
		return domain.ReplacementFinalizePreviewCommand(record.ID, lease.Generation)
	}
	return ""
}
