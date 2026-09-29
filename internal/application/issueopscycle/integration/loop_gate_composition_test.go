package cycleintegration

import (
	"issueops/internal/adapter/issueops"
	cycleapp "issueops/internal/application/issueopscycle"
	issueopscontract "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

// StrictPRReadiness는 레코드 기반 strict readiness에 loop gate를 더한다.
func StrictPRReadiness(record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
	return WithLoopGate(testCycleReadiness().StrictPR(record), record.Repo)
}

// StrictPRReadinessWithState는 state까지 읽는 strict readiness에 loop gate를 더한다.
func StrictPRReadinessWithState(stateRoot string, record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
	return WithLoopGate(testCycleReadiness().StrictPRWithState(stateRoot, record), record.Repo)
}

// AdvancePhase는 pr 단계 진입 전에 strict readiness를 강제한다.
func AdvancePhase(stateRoot, id, to string) (issueopscontract.IssueOpsRecord, error) {
	if err := guardPRPhase(stateRoot, id, to); err != nil {
		return issueopscontract.IssueOpsRecord{OK: false}, err
	}
	return advancePhaseForTest(stateRoot, id, to)
}

// AdvancePhaseWithActor는 AdvancePhase와 같은 gate를 actor 경로에 적용한다.
func AdvancePhaseWithActor(stateRoot, id, to string, actor issueops.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
	if err := guardPRPhase(stateRoot, id, to); err != nil {
		return issueopscontract.IssueOpsRecord{OK: false}, err
	}
	return advancePhaseWithActorForTest(stateRoot, id, to, actor)
}

// guardPRPhase는 이미 pr 단계인 레코드는 통과시킨다. 재진입까지 막으면 복구 경로가
// 사라지기 때문이다. core readiness는 AdvanceIssueOpsPhase가 upstream을 span 밖에서
// fetch한 뒤 span 안에서 판정하므로, 여기서는 이 package가 더하는 loop gate만
// 본다. core 판정을 여기서 다시 하면 upstream fetch가 두 번 일어난다.
func guardPRPhase(stateRoot, id, to string) error {
	return cycleapp.GuardPRPhase(stateRoot, id, to, cycleport.PRPhaseGuard{
		Read: issueops.ReadIssueOps,
		Gate: func(record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
			return WithLoopGate(issueopscontract.IssueOpsReadiness{Ready: true}, record.Repo)
		},
	})
}

// WithLoopGate는 readiness에 repo의 loop run gate를 더한다.
func WithLoopGate(ready issueopscontract.IssueOpsReadiness, repo string) issueopscontract.IssueOpsReadiness {
	return cycleapp.ApplyLoopGate(ready, repo, testLoopRepoGateMissing)
}
