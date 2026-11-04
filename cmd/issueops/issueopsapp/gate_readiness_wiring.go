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
	return app.GateService{BaseReadiness: func(root string, record model.Record) model.Readiness {
		return app.ApplyLoopGate(readiness.StrictPRWithState(root, record), record.Repo, loops.RepoGateMissing)
	},
		LoopReadiness: func(repo string) model.Readiness {
			return app.ApplyLoopGate(model.Readiness{Ready: true}, repo, loops.RepoGateMissing)
		},
		ReadRecord: issueopsadapter.ReadIssueOps, AdvanceRecord: func(root, id, to string, actor model.Actor) (model.Record, model.TrackedMaterials, error) {
			return newCyclePhaseService(&actor).AdvanceReport(root, id, to)
		},
		Ledger: cycleport.GateLedgerReadiness{Discover: gatesadapter.DiscoverGateFiles, Check: gates.Check}}
}
