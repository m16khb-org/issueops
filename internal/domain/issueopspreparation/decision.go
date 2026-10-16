// Package issueopspreparation owns the pure execution-preparation decision matrix.
package issueopspreparation

import (
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

type Code string

const (
	CodePreview          Code = "preview"
	CodeApply            Code = "apply"
	CodeExisting         Code = "existing"
	CodePendingReconcile Code = "pending_reconcile"
	CodeWriterless       Code = "writerless"
)

type DecisionInput struct {
	Command  preparationcontract.Command
	Snapshot preparationcontract.Snapshot
}

type Decision struct {
	Code Code
}

// Decide는 prepare 요청이 새 workspace를 만들지(preview/apply), 이미 준비된
// execution을 그대로 돌려줄지, 외부 intent나 writer 부재로 막을지 고른다.
func Decide(input DecisionInput) Decision {
	execution := input.Snapshot.Record.Execution
	if execution == nil {
		if input.Command.Confirm {
			return Decision{Code: CodeApply}
		}
		return Decision{Code: CodePreview}
	}
	switch {
	case execution.Pending != nil:
		return Decision{Code: CodePendingReconcile}
	case input.Command.Confirm && writerless(execution.Lease.Status):
		return Decision{Code: CodeWriterless}
	default:
		return Decision{Code: CodeExisting}
	}
}

func writerless(status string) bool {
	switch status {
	case "claimable", "released", "revoking":
		return true
	default:
		return false
	}
}
