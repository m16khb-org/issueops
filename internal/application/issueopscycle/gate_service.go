package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/domain/issueopsremote"
	cycleport "issueops/internal/port/issueopscycle"
)

// GateService 타입은 기본/루프 준비 상태에 게이트 원장 판정을 얹어 strict PR 준비 상태와 pr phase 전이 보호를 제공한다.
type GateService struct {
	BaseReadiness func(string, model.Record) model.Readiness
	LoopReadiness func(string) model.Readiness
	ReadRecord    func(string, string) (model.Record, error)
	AdvanceRecord func(string, string, string, model.Actor) (model.Record, model.TrackedMaterials, error)
	Ledger        cycleport.GateLedgerReadiness
}

// StrictPRReadinessWithState 메서드는 기본 PR 준비 상태에 게이트 원장 판정을 합쳐 strict 준비 상태를 돌려준다.
func (service GateService) StrictPRReadinessWithState(stateRoot string, record model.Record) model.Readiness {
	return service.apply(service.BaseReadiness(stateRoot, record), record)
}

// AdvancePhaseReport 메서드는 pr phase로 가는 전이이면 루프 준비 상태와 게이트 원장 판정을 먼저 통과해야 하고, 통과하면 AdvanceRecord로 전이한다.
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
	return applyGateLedgers(ready, root, number, service.Ledger)
}
