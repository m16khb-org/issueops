package mcpcli

import (
	lifecyclefiles "issueops/internal/adapter/lifecycle"
	"issueops/internal/adapter/lifecycle/fingerprint"
	lifecyclemodel "issueops/internal/adapter/lifecycle/model"
	statestore "issueops/internal/adapter/outbound/state"
	bootstrapfiles "issueops/internal/adapter/projectbootstrap"
	"issueops/internal/adapter/projectdoc"
	"issueops/internal/adapter/projectdocs"
	"issueops/internal/adapter/repopath"
	lifecycleapp "issueops/internal/application/lifecycle"
	bootstrapapp "issueops/internal/application/projectbootstrap"
	lifecyclecontract "issueops/internal/contract/lifecycle"
)

func testLifecycleService() lifecycleapp.Service {
	return lifecycleapp.Service{SchemaVersion: lifecyclemodel.ProjectLifecycleSchemaVersion, Effects: lifecyclefiles.ProfileFiles{
		Normalize: repopath.NormalizeRoot, StateDir: statestore.StateDir(),
		ObserveFingerprint: func(root string) lifecyclecontract.ProjectFingerprint {
			return fingerprint.ForRoot(root, projectdocs.ReadGitOriginURL)
		},
	}}
}

func testBootstrapService() bootstrapapp.Service {
	return bootstrapapp.Service{NormalizeRoot: repopath.NormalizeRoot, Effects: bootstrapfiles.Files{
		AnalyzeRepo: projectdocs.AnalyzeProjectSignals, InitializeLifecycle: testLifecycleService().Init,
		RenderDocs: projectdocs.RenderProjectDocs, RenderAgentsBlock: projectdocs.RenderAgentsWithBlock,
		FileAction: projectdoc.PlannedFileAction,
	}}
}

func testProjectDependencies() MCPDependencies {
	deps := testTransportServices()
	deps.ProjectDocs = testProjectDocsService()
	deps.ProjectBootstrap = testBootstrapService()
	return deps
}
