package issueopsapp

import (
	app "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
)

func planningRecorderForTest(actor *model.IssueOpsActor) app.PlanningRecorder {
	return newPlanningRecorder(actor)
}
