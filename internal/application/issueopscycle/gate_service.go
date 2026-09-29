package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
	cycleport "issueops/internal/port/issueopscycle"
)

type GateService struct {
	BaseReadiness  func(string, model.IssueOpsRecord) model.IssueOpsReadiness
	LoopReadiness  func(string) model.IssueOpsReadiness
	ReadRecord     func(string, string) (model.IssueOpsRecord, error)
	AdvanceRecord  func(string, string, string, model.IssueOpsActor) (model.IssueOpsRecord, error)
	Ledger         cycleport.GateLedgerReadiness
	DuplicateFiles func(string, string) (bool, []domain.GateLedgerFile)
}

func (service GateService) StrictPRReadinessWithState(stateRoot string, record model.IssueOpsRecord) model.IssueOpsReadiness {
	return service.apply(service.BaseReadiness(stateRoot, record), record)
}
func (service GateService) AdvancePhaseWithActor(stateRoot, id, to string, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	err := GuardPRPhase(stateRoot, id, to, cycleport.PRPhaseGuard{Read: service.ReadRecord, Gate: func(record model.IssueOpsRecord) model.IssueOpsReadiness {
		return service.apply(service.LoopReadiness(record.Repo), record)
	}})
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	return service.AdvanceRecord(stateRoot, id, to, actor)
}
func (service GateService) apply(ready model.IssueOpsReadiness, record model.IssueOpsRecord) model.IssueOpsReadiness {
	preparedURL := ""
	if record.BranchPrepare != nil {
		preparedURL = record.BranchPrepare.IssueURL
	}
	root, number := domain.PlanExistenceRoot(record), remote.GateLedgerIssueNumber(record.IssueURL, preparedURL)
	ready = ApplyGateLedgers(ready, root, number, service.Ledger)
	root, number, inspect := domain.DuplicateGateLedgerProbe(root, number)
	if !inspect {
		return ready
	}
	exists, entries := service.DuplicateFiles(root, number)
	return domain.ApplyDuplicateGateLedger(ready, number, exists, entries)
}
