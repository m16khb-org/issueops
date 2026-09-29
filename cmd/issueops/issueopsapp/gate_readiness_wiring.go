package issueopsapp

import (
	gatesadapter "issueops/internal/adapter/gates"
	issueopsadapter "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/issueops/gatesgate"
	app "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func newGateReadiness() app.GateService {
	gates := newGatesService()
	loops := newLoopReader()
	return app.GateService{BaseReadiness: func(root string, record model.IssueOpsRecord) model.IssueOpsReadiness {
		return app.ApplyLoopGate(issueopsadapter.IssueOpsStrictPRReadinessWithState(root, record), record.Repo, loops.RepoGateMissing)
	},
		LoopReadiness: func(repo string) model.IssueOpsReadiness {
			return app.ApplyLoopGate(model.IssueOpsReadiness{Ready: true}, repo, loops.RepoGateMissing)
		},
		ReadRecord: issueopsadapter.ReadIssueOps, AdvanceRecord: issueopsadapter.AdvanceIssueOpsPhaseWithActor,
		Ledger: cycleport.GateLedgerReadiness{Discover: gatesadapter.DiscoverGateFiles, Check: gates.Check}, DuplicateFiles: gatesgate.Observer{}.DuplicateFiles}
}
