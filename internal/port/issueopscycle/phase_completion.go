package issueopscycle

import model "issueops/internal/contract/issueops"

type PhaseCompletionReadiness struct {
	Compatibility         func(model.IssueOpsRecord) model.IssueOpsReadiness
	AISlopClean           func(model.IssueOpsRecord) model.IssueOpsReadiness
	PR                    func(model.IssueOpsRecord) model.IssueOpsReadiness
	RemoteArtifactMissing func(model.IssueOpsRecord) []string
}
