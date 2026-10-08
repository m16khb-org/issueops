package omp

import (
	"issueops/internal/adapter/hostprotocol"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	"issueops/internal/adapter/installutil"
	"issueops/internal/port"
)

func testDependencies() Dependencies {
	return Dependencies{
		MergeJSONMapFile:                installutil.MergeJSONMapFile,
		RemoveJSONMapEntry:              installutil.RemoveJSONMapEntry,
		VerifyJSONMapEntry:              installutil.VerifyJSONMapEntry,
		CaptureNativeActivationEvidence: installutil.CaptureNativeActivationEvidence,
		MCPCatalogSHA256:                func() (string, error) { return installutil.SemanticSHA256(mcpcatalog.AdvertisedTools()) },
		NewInstallPlan:                  func(host string, dry bool) port.InstallPlan { return installutil.NewPlan(host, dry) },
		PlanHostSkillLinks:              installutil.PlanHostSkillLinks,
		SemanticSHA256:                  installutil.SemanticSHA256,
		WriteJSONPlan:                   installutil.WriteJSONPlan,
		WriteTextPlan:                   installutil.WriteTextPlan,
	}
}
func testInstaller() Installer {
	return NewInstaller(testDependencies(), hostprotocol.OmpLifecycleExtension)
}
