package issueopscycle

import model "issueops/internal/contract/issueops"

type ReadinessObservations struct {
	WorktreePathValid    func(string) bool
	PlanPathExists       func(root, path string) bool
	PlanInLinkedWorktree func(model.IssueOpsRecord) bool
	WorkspaceMatches     func(left, right string) bool
	LinkedPlanDigest     func(model.IssueOpsRecord) (string, error)
	// SealedArchitectureDigest는 봉인된 architecture 문서의 sha256이다. 문서가 없으면 오류다.
	SealedArchitectureDigest func(model.IssueOpsRecord) (string, error)
}
