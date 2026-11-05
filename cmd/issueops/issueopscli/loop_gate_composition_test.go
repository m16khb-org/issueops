package issueopscli

import (
	core "issueops/internal/adapter/issueops"
	app "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func withLoopGateForTest(ready model.Readiness, repo string) model.Readiness {
	return app.ApplyLoopGate(ready, repo, testLoopRepoGateMissing)
}
func strictLoopReadinessForTest(root string, record model.Record) model.Readiness {
	return withLoopGateForTest(testCycleReadiness().StrictPRWithState(root, record), record.Repo)
}
func advanceLoopPhaseForTest(root, id, to string, actor model.Actor) (model.Record, model.TrackedMaterials, error) {
	if err := app.GuardPRPhase(root, id, to, cycleport.PRPhaseGuard{Read: core.ReadIssueOps, Gate: func(record model.Record) model.Readiness {
		return withLoopGateForTest(model.Readiness{Ready: true}, record.Repo)
	}}); err != nil {
		return model.Record{OK: false}, model.TrackedMaterials{}, err
	}
	return testCyclePhaseService(&actor).AdvanceReport(root, id, to)
}
