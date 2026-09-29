package contractcli

import (
	clicatalog "issueops/internal/adapter/inbound/catalog/cli"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	mcpcontract "issueops/internal/contract/mcp"
)

func testConformanceCatalog() []mcpcontract.Tool { return mcpcatalog.AdvertisedTools() }

func init() {
	ConfigureConformance(ConformanceDependencies{Catalog: testConformanceCatalog})
}

func testRunContract(args []string) error {
	return Run(args, clicatalog.Commands(), mcpcatalog.Build().Tools)
}
func testCompatibilityContract() CompatibilityContract {
	return BuildCompatibilityContract(clicatalog.Commands(), mcpcatalog.Build().Tools)
}
