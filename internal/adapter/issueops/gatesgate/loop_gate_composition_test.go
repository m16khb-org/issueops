package gatesgate

import (
	app "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
)

func withLoopGateForTest(ready model.IssueOpsReadiness, repo string) model.IssueOpsReadiness {
	return app.ApplyLoopGate(ready, repo, testLoopRepoGateMissing)
}
func strictLoopReadinessForTest(root string, record model.IssueOpsRecord) model.IssueOpsReadiness {
	return withLoopGateForTest(testCycleReadiness().StrictPRWithState(root, record), record.Repo)
}
