package projectbootstrap

import (
	lifecyclefiles "issueops/internal/adapter/lifecycle"
	"issueops/internal/adapter/lifecycle/fingerprint"
	lifecyclemodel "issueops/internal/adapter/lifecycle/model"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/projectdoc"
	"issueops/internal/adapter/projectdocs"
	"issueops/internal/adapter/repopath"
	lifecycleapp "issueops/internal/application/lifecycle"
	bootstrapapp "issueops/internal/application/projectbootstrap"
	lifecyclecontract "issueops/internal/contract/lifecycle"
	projectbootstrapcontract "issueops/internal/contract/projectbootstrap"
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
	return bootstrapapp.Service{NormalizeRoot: repopath.NormalizeRoot, Effects: Files{
		AnalyzeRepo: projectdocs.AnalyzeProjectSignals, InitializeLifecycle: testLifecycleService().Init,
		RenderDocs: projectdocs.RenderProjectDocs, RenderAgentsBlock: projectdocs.RenderAgentsWithBlock,
		FileAction: projectdoc.PlannedFileAction,
	}}
}

func BootstrapProjectDocs(request projectbootstrapcontract.ProjectDocsBootstrapRequest) (projectbootstrapcontract.ProjectDocsBootstrapResult, error) {
	return testBootstrapService().Run(request)
}
