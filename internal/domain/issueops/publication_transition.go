package issueops

import model "issueops/internal/contract/issueops"

// The caller validates publication authority before applying these transitions.
// Publication never completes the execution or releases its write lease.
func BeginPublication(record model.IssueOpsRecord, pending model.ExternalIntent) model.IssueOpsRecord {
	execution := *record.Execution
	execution.Pending = &pending
	execution.Failure = nil
	record.Execution = &execution
	return record
}

func FailPublication(record model.IssueOpsRecord, operationID, message, at string, notInvoked bool) model.IssueOpsRecord {
	execution := *record.Execution
	code := "external_operation_ambiguous"
	if notInvoked {
		execution.Pending = nil
		code = "external_operation_not_invoked"
	}
	execution.Failure = &model.ExecutionFailure{OperationID: operationID, Code: code, Message: message, At: at}
	record.Execution = &execution
	return record
}

func CompletePublication(record model.IssueOpsRecord, artifact model.IssueOpsRemoteArtifactVerification) model.IssueOpsRecord {
	execution := *record.Execution
	execution.Pending = nil
	execution.Failure = nil
	record.Execution = &execution
	record.RemoteArtifact = &artifact
	return record
}
