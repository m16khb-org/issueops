package issueopscli

import model "issueops/internal/contract/issueops"

type LoopGateDeps struct {
	AdvancePhaseWithActor      func(stateRoot, id, to string, actor model.IssueOpsActor) (model.IssueOpsRecord, error)
	StrictPRReadinessWithState func(stateRoot string, record model.IssueOpsRecord) model.IssueOpsReadiness
}
