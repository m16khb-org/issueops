package issueopspreparation

import (
	"fmt"
	"strings"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func MarkOrcaInvoking(intent preparationcontract.Intent) preparationcontract.Intent {
	intent.InvocationState = preparationcontract.InvocationUnknown
	intent.InvocationAttempts++
	return intent
}

func ApplyOrcaFailure(state IntentState, invocation string, diagnostic func() string) (leasecontract.Record, preparationcontract.Intent, error) {
	if strings.TrimSpace(state.FailureAt) == "" {
		return leasecontract.Record{}, preparationcontract.Intent{}, fmt.Errorf("Orca intent failure timestamp is required")
	}
	intent := state.Intent
	intent.InvocationState = invocation
	record := state.Snapshot.Record
	execution := *record.Execution
	record.Execution = &execution
	record.Execution.Failure = &leasecontract.FailureDetail{
		OperationID: intent.OperationID, Code: "external_operation_ambiguous",
		Message: diagnostic(), At: state.FailureAt,
	}
	return record, intent, nil
}

func ApplyOrcaNoResource(state IntentState, diagnostic func() string) (leasecontract.Record, error) {
	if strings.TrimSpace(state.FailureAt) == "" {
		return leasecontract.Record{}, fmt.Errorf("Orca intent failure timestamp is required")
	}
	record := state.Snapshot.Record
	execution := *record.Execution
	record.Execution = &execution
	record.Execution.Pending = nil
	record.Execution.Failure = &leasecontract.FailureDetail{
		OperationID: state.Intent.OperationID, Code: "external_operation_left_no_resource",
		Message: diagnostic(), At: state.FailureAt,
	}
	return record, nil
}
