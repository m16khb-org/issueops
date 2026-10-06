package issueopscli

import clicatalog "issueops/internal/adapter/inbound/catalog/cli"

func testCLIUsageDependencies() Dependencies {
	return Dependencies{Remote: testRemoteCommand(), Cleanup: testCleanupCommand(), CleanupRuntime: testCleanupRuntime(), Benchmark: testBenchmarkCommand(), Usage: clicatalog.LifecycleUsage(), ChildUsage: clicatalog.ChildUsage()}
}
func runIssueOps(args []string) error {
	return runIssueOpsForTest(args, testCLIUsageDependencies())
}
