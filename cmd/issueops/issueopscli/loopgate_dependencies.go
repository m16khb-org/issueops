package issueopscli

import model "issueops/internal/contract/issueops"

type LoopGateDeps struct {
	AdvancePhaseReport         func(stateRoot, id, to string, actor model.Actor) (model.Record, model.TrackedMaterials, error)
	StrictPRReadinessWithState func(stateRoot string, record model.Record) model.Readiness
}
