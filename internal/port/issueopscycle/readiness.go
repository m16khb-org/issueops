package issueopscycle

import model "issueops/internal/contract/issueops"

type ReadinessObservations struct {
	WorktreePathValid    func(string) bool
	PlanPathExists       func(root, path string) bool
	PlanInLinkedWorktree func(model.IssueOpsRecord) bool
	WorkspaceMatches     func(left, right string) bool
	LinkedPlanDigest     func(model.IssueOpsRecord) (string, error)
}
