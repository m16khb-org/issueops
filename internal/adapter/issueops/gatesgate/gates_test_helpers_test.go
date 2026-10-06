package gatesgate

import (
	adapter "issueops/internal/adapter/gates"
	"issueops/internal/adapter/issueops"
	policyadapter "issueops/internal/adapter/policy"
	app "issueops/internal/application/gates"
	cycleapp "issueops/internal/application/issueopscycle"
	policyapp "issueops/internal/application/policy"
	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func readinessServiceForTest() cycleapp.GateService {
	policy := policyapp.Service{Observer: policyadapter.CommandObserver{}, Overrides: policyadapter.OverrideLoader{}, Executor: policyadapter.CommandExecutor{}, Clock: policyadapter.Clock{}}
	gates := app.Service{Store: adapter.FileStore{}, Clock: adapter.Clock{}, Runner: app.CommandRunner{Evaluate: policy.Evaluate, Execute: policy.Run}}
	return cycleapp.GateService{BaseReadiness: strictLoopReadinessForTest,
		LoopReadiness: func(repo string) model.IssueOpsReadiness {
			return withLoopGateForTest(model.IssueOpsReadiness{Ready: true}, repo)
		},
		ReadRecord: issueops.ReadIssueOps, AdvanceRecord: func(root, id, to string, actor model.IssueOpsActor) (model.IssueOpsRecord, model.IssueOpsTrackedMaterials, error) {
			return testCyclePhaseService(&actor).AdvanceReport(root, id, to)
		},
		Ledger: cycleport.GateLedgerReadiness{Discover: adapter.DiscoverGateFiles, Check: gates.Check}, DuplicateFiles: Observer{}.DuplicateFiles}
}
func StrictPRReadinessWithState(root string, record model.IssueOpsRecord) model.IssueOpsReadiness {
	return readinessServiceForTest().StrictPRReadinessWithState(root, record)
}
func AdvancePhaseWithActor(root, id, to string, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	record, _, err := readinessServiceForTest().AdvancePhaseReport(root, id, to, actor)
	return record, err
}
