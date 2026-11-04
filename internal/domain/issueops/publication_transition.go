package issueops

import model "issueops/internal/contract/issueops"

// The caller validates publication authority before applying these transitions.
// Publication never completes the execution or releases its write lease.
func BeginPublication(record model.Record, pending model.ExternalIntent) model.Record {
	execution := *record.Execution
	execution.Pending = &pending
	execution.Failure = nil
	record.Execution = &execution
	return record
}

func FailPublication(record model.Record, operationID, message, at string, notInvoked bool) model.Record {
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

func CompletePublication(record model.Record, artifact model.RemoteArtifactVerification) model.Record {
	execution := *record.Execution
	execution.Pending = nil
	execution.Failure = nil
	record.Execution = &execution
	record.RemoteArtifact = &artifact
	return record
}
