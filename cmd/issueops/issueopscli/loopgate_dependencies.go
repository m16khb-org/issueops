package issueopscli

import model "issueops/internal/contract/issueops"

// LoopGateDeps 타입은 loop gate가 단계 전환과 PR 준비 상태 판정에 쓰는 연산이다.
type LoopGateDeps struct {
	AdvancePhaseReport         func(stateRoot, id, to string, actor model.Actor) (model.Record, model.TrackedMaterials, error)
	StrictPRReadinessWithState func(stateRoot string, record model.Record) model.Readiness
}
