package issueopsapp

import (
	gatesadapter "issueops/internal/adapter/gates"
	issueopsadapter "issueops/internal/adapter/issueops"
	app "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func newGateReadiness() app.GateService {
	gates := newGatesService()
	loops := newLoopReader()
	readiness := newCycleReadiness()
	return app.GateService{BaseReadiness: func(root string, record model.IssueOpsRecord) model.IssueOpsReadiness {
		return app.ApplyLoopGate(readiness.StrictPRWithState(root, record), record.Repo, loops.RepoGateMissing)
	},
		LoopReadiness: func(repo string) model.IssueOpsReadiness {
			return app.ApplyLoopGate(model.IssueOpsReadiness{Ready: true}, repo, loops.RepoGateMissing)
		},
		ReadRecord: issueopsadapter.ReadIssueOps, AdvanceRecord: func(root, id, to string, actor model.IssueOpsActor) (model.IssueOpsRecord, model.IssueOpsTrackedMaterials, error) {
			return newCyclePhaseService(&actor).AdvanceReport(root, id, to)
		},
		Ledger: cycleport.GateLedgerReadiness{Discover: gatesadapter.DiscoverGateFiles, Check: gates.Check}}
}
