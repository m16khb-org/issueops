package gatesgate

import (
	adapter "issueops/internal/adapter/gates"
	"issueops/internal/adapter/issueops"
	policyadapter "issueops/internal/adapter/policy"
	app "issueops/internal/application/gates"
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func readinessServiceForTest() cycleapp.GateService {
	gates := app.Service{Store: adapter.FileStore{}, Clock: adapter.Clock{}, Runner: app.CommandRunner{Evaluate: policyadapter.EvaluateCommandPolicy, Execute: policyadapter.RunCommand}}
	return cycleapp.GateService{BaseReadiness: strictLoopReadinessForTest,
		LoopReadiness: func(repo string) model.IssueOpsReadiness {
			return withLoopGateForTest(model.IssueOpsReadiness{Ready: true}, repo)
		},
		ReadRecord: issueops.ReadIssueOps, AdvanceRecord: issueops.AdvanceIssueOpsPhaseWithActor,
		Ledger: cycleport.GateLedgerReadiness{Discover: adapter.DiscoverGateFiles, Check: gates.Check}, DuplicateFiles: Observer{}.DuplicateFiles}
}
func StrictPRReadinessWithState(root string, record model.IssueOpsRecord) model.IssueOpsReadiness {
	return readinessServiceForTest().StrictPRReadinessWithState(root, record)
}
func AdvancePhaseWithActor(root, id, to string, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return readinessServiceForTest().AdvancePhaseWithActor(root, id, to, actor)
}
