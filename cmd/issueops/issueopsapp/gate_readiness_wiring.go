package issueopsapp

import (
	gatesadapter "issueops/internal/adapter/gates"
	issueopsadapter "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/issueops/gatesgate"
	"issueops/internal/adapter/issueops/loopgate"
	app "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func newGateReadiness() app.GateService {
	gates := newGatesService()
	return app.GateService{BaseReadiness: loopgate.StrictPRReadinessWithState,
		LoopReadiness: func(repo string) model.IssueOpsReadiness {
			return loopgate.WithLoopGate(model.IssueOpsReadiness{Ready: true}, repo)
		},
		ReadRecord: issueopsadapter.ReadIssueOps, AdvanceRecord: issueopsadapter.AdvanceIssueOpsPhaseWithActor,
		Ledger: cycleport.GateLedgerReadiness{Discover: gatesadapter.DiscoverGateFiles, Check: gates.Check}, DuplicateFiles: gatesgate.Observer{}.DuplicateFiles}
}
