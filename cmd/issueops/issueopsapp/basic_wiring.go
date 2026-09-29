package issueopsapp

import (
	"os"

	"issueops/cmd/issueops/basiccli"
	docsadapter "issueops/internal/adapter/docs"
	guardadapter "issueops/internal/adapter/guard"
	issueopsadapter "issueops/internal/adapter/issueops"
	preflightadapter "issueops/internal/adapter/preflight"
	guardapp "issueops/internal/application/guard"
	preflightapp "issueops/internal/application/preflight"
)

func newBasicCommand() basiccli.Command {
	cwd, _ := os.Getwd()
	return basiccli.Command{
		IssueOpsRoot: issueOpsRoot(), DefaultTarget: resolveTarget(""), Version: version,
		DocsIndex: docsadapter.DocsIndex, InspectHarness: newHarnessInspector(),
		Preflight: preflightapp.Service{Observer: preflightadapter.GitObserver{}},
		Guard:     guardapp.Service{Source: guardadapter.Source{BaseDir: cwd}},
		Trace:     newTraceService(), Handoff: newHandoffDeliveryService(issueopsadapter.IssueOpsStateRoot()),
	}
}
