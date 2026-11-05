// Package issueopspreparation owns the pure execution-preparation decision matrix.
package issueopspreparation

import (
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

type code string

// Decision codes. The unexported codes are the preview and apply paths that
// create a workspace; the exported ones are the outcomes callers must handle
// when an execution already exists.
const (
	codePreview          code = "preview"
	codeApply            code = "apply"
	CodeExisting         code = "existing"
	CodePendingReconcile code = "pending_reconcile"
	CodeWriterless       code = "writerless"
)

// DecisionInput is what Decide reads: the prepare command and the current
// record snapshot.
type DecisionInput struct {
	Command  preparationcontract.Command
	Snapshot preparationcontract.Snapshot
}

// Decision is the outcome of Decide; Code selects the prepare path.
type Decision struct {
	Code code
}

// Decide 함수는 prepare 요청이 새 workspace를 만들지(preview/apply), 이미 준비된
// execution을 그대로 돌려줄지, 외부 intent나 writer 부재로 막을지 고른다.
func Decide(input DecisionInput) Decision {
	execution := input.Snapshot.Record.Execution
	if execution == nil {
		if input.Command.Confirm {
			return Decision{Code: codeApply}
		}
		return Decision{Code: codePreview}
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
