package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

// ApplyLoopGate 함수는 observe가 repo에서 관찰한 누락·경고를 준비 상태에 합친다.
func ApplyLoopGate(ready model.Readiness, repo string, observe func(string) ([]string, []string)) model.Readiness {
	missing, warnings := observe(repo)
	return domain.MergeGateReadiness(ready, missing, warnings)
}

// GuardPRPhase 함수는 to가 pr 게이트를 읽어야 하는 phase일 때 기록의 게이트 판정을 평가해 막히면 오류를 돌려준다.
func GuardPRPhase(stateRoot, id, to string, ports cycleport.PRPhaseGuard) error {
	if !domain.NeedsPRGateRead(to) {
		return nil
	}
	record, err := ports.Read(stateRoot, id)
	if err != nil {
		return err
	}
	return domain.PRGateError(ports.Gate(record))
}
