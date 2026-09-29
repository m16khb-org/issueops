package issueopscli

import (
	core "issueops/internal/adapter/issueops"
	app "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func withLoopGateForTest(ready model.IssueOpsReadiness, repo string) model.IssueOpsReadiness {
	return app.ApplyLoopGate(ready, repo, testLoopRepoGateMissing)
}
func strictLoopReadinessForTest(root string, record model.IssueOpsRecord) model.IssueOpsReadiness {
	return withLoopGateForTest(testCycleReadiness().StrictPRWithState(root, record), record.Repo)
}
func advanceLoopPhaseForTest(root, id, to string, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	if err := app.GuardPRPhase(root, id, to, cycleport.PRPhaseGuard{Read: core.ReadIssueOps, Gate: func(record model.IssueOpsRecord) model.IssueOpsReadiness {
		return withLoopGateForTest(model.IssueOpsReadiness{Ready: true}, record.Repo)
	}}); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	return advancePhaseWithActorForTest(root, id, to, actor)
}
