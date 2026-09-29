package issueopscli

import (
	"context"
	adapter "issueops/internal/adapter/issueops"
	ownerapp "issueops/internal/application/issueopsowner"
	model "issueops/internal/contract/issueops"
)

var testIssueOpsRuntime IssueOpsCLIDeps

func testIssueOpsCommand() command {
	return command{Runtime: testIssueOpsRuntime, Gates: testIssueOpsGates, VerifyChild: testRemoteVerifier().Child}
}

func runIssueOpsForTest(args []string, deps Dependencies) error {
	if deps.Runtime.IssueOpsStateRoot == nil {
		deps.Runtime = testIssueOpsRuntime
	}
	if deps.Gates.AdvancePhaseWithActor == nil {
		deps.Gates = testIssueOpsGates
	}
	if deps.Status == nil {
		deps.Status = executionStatusForTest
	}
	if deps.Execution.ExecuteExecution == nil {
		deps.Execution = testCLIExecutionRuntime()
	}
	if deps.Verification.Verify == nil {
		child := deps.Verification.Child
		deps.Verification = testRemoteVerificationHandlers()
		if child != nil {
			deps.Verification.Child = child
		}
	}
	return RunIssueOpsWithDependencies(args, deps)
}

func executionStatusForTest(_ context.Context, root, id string) (model.ExecutionResult, error) {
	return (ownerapp.ExecutionStatus{ReadRecord: (adapter.CycleRecordStore{StateRoot: root}).Load}).Read(id)
}
