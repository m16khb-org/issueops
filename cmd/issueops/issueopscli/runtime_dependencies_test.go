package issueopscli

var testIssueOpsRuntime IssueOpsCLIDeps

func testIssueOpsCommand() command {
	return command{Runtime: testIssueOpsRuntime, Gates: testIssueOpsGates}
}

func runIssueOpsForTest(args []string, deps Dependencies) error {
	if deps.Runtime.IssueOpsStateRoot == nil {
		deps.Runtime = testIssueOpsRuntime
	}
	if deps.Gates.AdvancePhaseWithActor == nil {
		deps.Gates = testIssueOpsGates
	}
	return RunIssueOpsWithDependencies(args, deps)
}
