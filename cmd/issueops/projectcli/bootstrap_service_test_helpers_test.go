package projectcli

import (
	"fmt"
	"os"
	"testing"

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

// TestMain points the lifecycle state root at a scratch directory, so the
// bootstrap tests never read or write the developer's ~/.local/state.
func TestMain(m *testing.M) {
	stateDir, err := os.MkdirTemp("", "issueops-projectcli-state-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.Setenv("ISSUEOPS_STATE_DIR", stateDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(stateDir)
	os.Exit(code)
}

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

func testProjectDependencies() Dependencies {
	return Dependencies{Commit: testCommitService(), Lint: testLintService(), Docs: testProjectDocsService(), Bootstrap: testBootstrapService()}
}
