package gatesgate

import (
	core "issueops/internal/adapter/issueops"
	app "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
)

func withLoopGateForTest(ready model.IssueOpsReadiness, repo string) model.IssueOpsReadiness {
	return app.ApplyLoopGate(ready, repo, testLoopRepoGateMissing)
}
func strictLoopReadinessForTest(root string, record model.IssueOpsRecord) model.IssueOpsReadiness {
	return withLoopGateForTest(core.IssueOpsStrictPRReadinessWithState(root, record), record.Repo)
}
