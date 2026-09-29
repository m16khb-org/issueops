package issueopscli

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
