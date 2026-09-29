package mcpcli

import (
	"issueops/cmd/issueops/contractcli"
	clicatalog "issueops/internal/adapter/inbound/catalog/cli"
	"issueops/internal/adapter/inspect"
	webfetch "issueops/internal/adapter/outbound/webfetch"
	"issueops/internal/adapter/preflight"
	preflightapp "issueops/internal/application/preflight"
)

func testTransportServices() MCPDependencies {
	return MCPDependencies{DefaultTarget: IssueOpsRoot(), Preflight: preflightapp.Service{Observer: preflight.GitObserver{}}, Skills: inspect.ListSkills, Resources: resourceConfigForTest(), Compatibility: func() any {
		return contractcli.BuildCompatibilityContract(clicatalog.Commands(), testMCPCatalog().Tools)
	}, Commit: testCommitService(), Lint: testLintService(), Fetch: webfetch.Fetch, Worker: testWorkerService(), Daemon: testDaemonReader(), Execution: testExecutionDeps(), Catalog: testMCPCatalog()}
}
