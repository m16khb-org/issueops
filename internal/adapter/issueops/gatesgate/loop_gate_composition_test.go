package gatesgate

import (
	app "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
)

func withLoopGateForTest(ready model.Readiness, repo string) model.Readiness {
	return app.ApplyLoopGate(ready, repo, testLoopRepoGateMissing)
}
func strictLoopReadinessForTest(root string, record model.Record) model.Readiness {
	return withLoopGateForTest(testCycleReadiness().StrictPRWithState(root, record), record.Repo)
}
