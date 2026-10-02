package issueopsapp

import (
	lifecyclefiles "issueops/internal/adapter/lifecycle"
	"issueops/internal/adapter/lifecycle/fingerprint"
	lifecyclemodel "issueops/internal/adapter/lifecycle/model"
	statestore "issueops/internal/adapter/outbound/state"
	bootstrapfiles "issueops/internal/adapter/projectbootstrap"
	"issueops/internal/adapter/projectdoc"
	"issueops/internal/adapter/projectdocs"
	lifecycleapp "issueops/internal/application/lifecycle"
	bootstrapapp "issueops/internal/application/projectbootstrap"
	lifecyclecontract "issueops/internal/contract/lifecycle"
)

func newProjectLifecycleService() lifecycleapp.Service {
	return lifecycleapp.Service{SchemaVersion: lifecyclemodel.ProjectLifecycleSchemaVersion, Effects: lifecyclefiles.ProfileFiles{
		Normalize: newRepoRootResolver("."), StateDir: statestore.StateDir(),
		ObserveFingerprint: func(root string) lifecyclecontract.ProjectFingerprint {
			return fingerprint.ForRoot(root, projectdocs.ReadGitOriginURL)
		},
	}}
}

func newProjectBootstrapService(defaultRoot string) bootstrapapp.Service {
	return projectBootstrapServiceWith(newRepoRootResolver(defaultRoot))
}

func newScopedProjectBootstrapService(root, cwd string) bootstrapapp.Service {
	return projectBootstrapServiceWith(newRepoRootResolverAt(root, cwd, nil))
}

func projectBootstrapServiceWith(normalizeRoot func(string) (string, error)) bootstrapapp.Service {
	lifecycle := newProjectLifecycleService()
	return bootstrapapp.Service{NormalizeRoot: normalizeRoot, Effects: bootstrapfiles.Files{
		AnalyzeRepo: projectdocs.AnalyzeProjectSignals, InitializeLifecycle: lifecycle.Init,
		RenderDocs: projectdocs.RenderProjectDocs, RenderAgentsBlock: projectdocs.RenderAgentsWithBlock,
		FileAction: projectdoc.PlannedFileAction,
	}}
}
