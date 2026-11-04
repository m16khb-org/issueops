package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/domain/issueopsremote"
	cycleport "issueops/internal/port/issueopscycle"
)

type GateService struct {
	BaseReadiness func(string, model.Record) model.Readiness
	LoopReadiness func(string) model.Readiness
	ReadRecord    func(string, string) (model.Record, error)
	AdvanceRecord func(string, string, string, model.Actor) (model.Record, model.TrackedMaterials, error)
	Ledger        cycleport.GateLedgerReadiness
}

func (service GateService) StrictPRReadinessWithState(stateRoot string, record model.Record) model.Readiness {
	return service.apply(service.BaseReadiness(stateRoot, record), record)
}
func (service GateService) AdvancePhaseReport(stateRoot, id, to string, actor model.Actor) (model.Record, model.TrackedMaterials, error) {
	err := GuardPRPhase(stateRoot, id, to, cycleport.PRPhaseGuard{Read: service.ReadRecord, Gate: func(record model.Record) model.Readiness {
		return service.apply(service.LoopReadiness(record.Repo), record)
	}})
	if err != nil {
		return model.Record{OK: false}, model.TrackedMaterials{}, err
	}
	return service.AdvanceRecord(stateRoot, id, to, actor)
}
func (service GateService) apply(ready model.Readiness, record model.Record) model.Readiness {
	preparedURL := ""
	if record.BranchPrepare != nil {
		preparedURL = record.BranchPrepare.IssueURL
	}
	root, number := domain.PlanExistenceRoot(record), issueopsremote.GateLedgerIssueNumber(record.IssueURL, preparedURL)
	return ApplyGateLedgers(ready, root, number, service.Ledger)
}
