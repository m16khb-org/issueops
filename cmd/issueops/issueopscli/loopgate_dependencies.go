package issueopscli

import model "issueops/internal/contract/issueops"

type LoopGateDeps struct {
	AdvancePhaseReport         func(stateRoot, id, to string, actor model.IssueOpsActor) (model.IssueOpsRecord, model.IssueOpsTrackedMaterials, error)
	StrictPRReadinessWithState func(stateRoot string, record model.IssueOpsRecord) model.IssueOpsReadiness
}
