package contractcli

import (
	fixtureapp "issueops/internal/application/toolconformance"

	clicatalog "issueops/internal/adapter/inbound/catalog/cli"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	"issueops/internal/adapter/toolconformance"
)

func newTestConformance(deps ConformanceDependencies) *Conformance {
	if deps.Catalog == nil {
		deps.Catalog = mcpcatalog.AdvertisedTools
	}
	deps.LoadManifest = (fixtureapp.FixtureService{Files: toolconformance.FixtureFiles{}}).LoadManifest
	deps.LoadRegressionFixture = (fixtureapp.FixtureService{Files: toolconformance.FixtureFiles{}}).LoadRegressionFixture
	deps.ReplayRegression = (fixtureapp.FixtureService{Files: toolconformance.FixtureFiles{}}).ReplayRegression
	return NewConformance(deps)
}

func testRunContract(args []string) error {
	return Run(args, clicatalog.Commands(), mcpcatalog.Build().Tools, newTestConformance(ConformanceDependencies{}))
}
func testCompatibilityContract() CompatibilityContract {
	return BuildCompatibilityContract(clicatalog.Commands(), mcpcatalog.Build().Tools)
}
