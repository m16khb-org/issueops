package issueopscli

import clicatalog "issueops/internal/adapter/inbound/catalog/cli"

func testCLIUsageDependencies() Dependencies {
	return Dependencies{Usage: clicatalog.LifecycleUsage(), ChildUsage: clicatalog.ChildUsage()}
}
func runIssueOps(args []string) error {
	return runIssueOpsForTest(args, testCLIUsageDependencies())
}
func testLifecycleUsage() string { return clicatalog.LifecycleUsage() }
func testChildUsage() string     { return clicatalog.ChildUsage() }
