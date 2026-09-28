package issueopscycle

import model "issueops/internal/contract/issueops"

type PhaseEntryReadiness struct {
	Problem               func(model.IssueOpsRecord) model.IssueOpsReadiness
	Grill                 func(model.IssueOpsRecord) model.IssueOpsReadiness
	Plan                  func(model.IssueOpsRecord) model.IssueOpsReadiness
	Compatibility         func(model.IssueOpsRecord) model.IssueOpsReadiness
	Implement             func(model.IssueOpsRecord) model.IssueOpsReadiness
	AISlopClean           func(model.IssueOpsRecord) model.IssueOpsReadiness
	StrictPR              func(model.IssueOpsRecord) model.IssueOpsReadiness
	RemoteArtifactMissing func(model.IssueOpsRecord) []string
}
