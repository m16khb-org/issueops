package issueopsapp

import (
	"issueops/cmd/issueops/contractcli"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	mcpadapter "issueops/internal/adapter/mcp"
	"issueops/internal/adapter/toolconformance"
)

// contractcli는 fixture 저장소 구현을 알지 않는다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func newContractConformance() *contractcli.Conformance {
	return contractcli.NewConformance(contractcli.ConformanceDependencies{
		Catalog:               mcpcatalog.AdvertisedTools,
		Root:                  issueOpsRoot,
		RunProcess:            runToolConformanceLive,
		ServeProbe:            mcpadapter.ServeConformanceProbe,
		LoadManifest:          toolconformance.LoadManifest,
		LoadRegressionFixture: toolconformance.LoadRegressionFixture,
		ReplayRegression:      toolconformance.ReplayRegression,
	})
}
