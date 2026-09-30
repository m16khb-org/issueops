package issueopsapp

import (
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
		IssueOpsRoot: issueOpsRoot(), DefaultTarget: resolveTarget(""), Version: version,
		DocsIndex: newDocsService().Index, InspectHarness: newHarnessInspector(),
		Preflight: preflightapp.Service{Observer: preflightadapter.GitObserver{}},
		Guard:     guardapp.Service{Source: guardadapter.Source{BaseDir: cwd}},
		Trace:     newTraceService(), Handoff: newHandoffDeliveryService(issueOpsStateRoot()),
	}
}
