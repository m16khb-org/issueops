package issueopsapp

import (
	pathutil "issueops/cmd/issueops/pathutil"
	"os"

	"issueops/cmd/issueops/basiccli"

	guardadapter "issueops/internal/adapter/guard"

	preflightadapter "issueops/internal/adapter/preflight"
	guardapp "issueops/internal/application/guard"
	preflightapp "issueops/internal/application/preflight"
)

func newBasicCommand() basiccli.Command {
	cwd, _ := os.Getwd()
	return basiccli.Command{
		IssueOpsRoot: issueOpsRoot(), DefaultTarget: pathutil.ResolveTarget(""), Version: version,
		DocsIndex: newDocsService().Index, InspectHarness: newHarnessHostInspector(),
		Preflight: preflightapp.Service{Observer: preflightadapter.GitObserver{}},
		Guard:     guardapp.Service{Source: guardadapter.Source{BaseDir: cwd}},
		Trace:     newTraceService(), Handoff: newHandoffDeliveryService(issueOpsStateRoot()),
	}
}
