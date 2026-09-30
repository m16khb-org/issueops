package issueopscycle

import model "issueops/internal/contract/issueops"

type PRPhaseGuard struct {
	Read func(stateRoot, id string) (model.IssueOpsRecord, error)
	Gate func(model.IssueOpsRecord) model.IssueOpsReadiness
}
