package contractcli

import (
	clicatalog "issueops/internal/adapter/inbound/catalog/cli"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	"issueops/internal/adapter/toolconformance"
	mcpcontract "issueops/internal/contract/mcp"
)

func testConformanceCatalog() []mcpcontract.Tool { return mcpcatalog.AdvertisedTools() }

func newTestConformance(deps ConformanceDependencies) *Conformance {
	if deps.Catalog == nil {
		deps.Catalog = testConformanceCatalog
	}
	deps.LoadManifest = toolconformance.LoadManifest
	deps.LoadRegressionFixture = toolconformance.LoadRegressionFixture
	deps.ReplayRegression = toolconformance.ReplayRegression
	return NewConformance(deps)
}

func testRunContract(args []string) error {
	return Run(args, clicatalog.Commands(), mcpcatalog.Build().Tools, newTestConformance(ConformanceDependencies{}))
}
func testCompatibilityContract() CompatibilityContract {
	return BuildCompatibilityContract(clicatalog.Commands(), mcpcatalog.Build().Tools)
}
