package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func ApplyLoopGate(ready model.IssueOpsReadiness, repo string, observe func(string) ([]string, []string)) model.IssueOpsReadiness {
	missing, warnings := observe(repo)
	return domain.MergeGateReadiness(ready, missing, warnings)
}
func GuardPRPhase(stateRoot, id, to string, ports cycleport.PRPhaseGuard) error {
	if !domain.NeedsPRGateRead(to) {
		return nil
	}
	record, err := ports.Read(stateRoot, id)
	if err != nil {
		return err
	}
	if !domain.NeedsPRGateEvaluation(record.Phase) {
		return nil
	}
	return domain.PRGateError(ports.Gate(record))
}
