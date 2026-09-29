package issueops

import (
	"fmt"
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	leasedomain "issueops/internal/domain/issueopslease"
	"strconv"
)

func StatusExecution(stateRoot, id string) (ExecutionResult, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return ExecutionResult{OK: false, ID: id}, err
	}
	if record.Execution == nil {
		return ExecutionResult{OK: false, ID: id}, fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	result := executionResult(record)
	if record.Execution.Completion == nil {
		result.NextCommand = executionWriterAbsentRecoveryCommand(record)
	}
	return result, nil
}

// ExecutionWriterAbsentRecoveryCommand는 writer 없는 lease의 회복 명령을
// 노출한다. 본체는 아래 한 곳뿐이며, 단계 분류가 같은 문자열을 다시 만들지
// 않도록 감싸기만 한다.
func ExecutionWriterAbsentRecoveryCommand(record issueops.IssueOpsRecord) string {
	return executionWriterAbsentRecoveryCommand(record)
}

func executionWriterAbsentRecoveryCommand(record issueops.IssueOpsRecord) string {
	if record.Execution == nil {
		return ""
	}
	lease := record.Execution.Lease
	identityComplete := false
	if lease.Status == issueops.LeaseStatusClaimable && record.Execution.Mode == issueops.ExecutionModeOrca {
		identityComplete = completeOrcaArtifactIdentity(record.Execution.Orca)
	}
	switch leasedomain.DecideWriterlessRecovery(string(lease.Status), string(record.Execution.Mode), identityComplete) {
	case leasedomain.RecoveryReplacePreview:
		return executionReplacementPreviewCommand(record.ID, lease.Generation)
	case leasedomain.RecoveryResume:
		return domain.ReplacementResumeCommand(record.ID, lease.Generation)
	case leasedomain.RecoveryDirectClaim:
		return domain.ReplacementClaimCommand(record.ID, lease.Generation, claimTokenPath(record))
	case leasedomain.RecoveryFinalizePreview:
		return "issueops execution replace --id " + quoteExecutionOwnerArg(record.ID) +
			" --expected-generation " + strconv.FormatUint(lease.Generation, 10) + " --finalize-preview"
	}
	return ""
}

func executionReplacementPreviewCommand(id string, generation uint64) string {
	return "issueops execution replace --id " + quoteExecutionOwnerArg(id) +
		" --expected-generation " + strconv.FormatUint(generation, 10) + " --preview"
}

func executionResult(record issueops.IssueOpsRecord) ExecutionResult {
	return ExecutionResult{OK: true, ID: record.ID, Execution: *record.Execution}
}
