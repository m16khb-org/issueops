package mcpcli

import (
	"issueops/cmd/issueops/contractcli"
	clicatalog "issueops/internal/adapter/inbound/catalog/cli"
	"issueops/internal/adapter/inspect"
	"issueops/internal/adapter/preflight"
	preflightapp "issueops/internal/application/preflight"
)

func testTransportServices() MCPDependencies {
	return MCPDependencies{APIDoc: testAPIDocService(), DefaultTarget: IssueOpsRoot(), Preflight: preflightapp.Service{Observer: preflight.GitObserver{}}, Skills: inspect.ListSkills, Resources: resourceConfigForTest(), Compatibility: func() any {
		return contractcli.BuildCompatibilityContract(clicatalog.Commands(), testMCPCatalog().Tools)
	}, Commit: testCommitService(), Lint: testLintService(), Fetch: testWebFetch, Worker: testWorkerService(), Execution: testExecutionDeps(), Catalog: testMCPCatalog()}
}
