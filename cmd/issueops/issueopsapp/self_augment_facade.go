package issueopsapp

import (
	"issueops/cmd/issueops/selfworkflow/augmentcmd"
	"issueops/cmd/issueops/selfworkflow/augmentlesson"
	statestore "issueops/internal/adapter/outbound/state"
	augmentcontract "issueops/internal/contract/selfaugment"
)

func runSelfAugment(args []string) error {
	return augmentcmd.Run(args, augmentcmd.Deps{
		RunLesson: runSelfAugmentLesson,
		RunVerify: runSelfVerify,
		Plan:      planSelfAugmentation,
		SavePlan:  newSelfWorkflowState(statestore.StateDir()).SavePlan,
		PrintJSON: printJSON,
	})
}

func planSelfAugmentation(req augmentcontract.SelfAugmentPlanRequest) augmentcontract.SelfAugmentPlanResult {
	return newSelfWorkflowPlanning(issueOpsRoot(), statestore.StateDir(), version).Plan(req)
}

func runSelfAugmentLesson(args []string) error {
	return augmentlesson.Run(args, augmentlesson.Deps{Save: newSelfWorkflowPlanning(issueOpsRoot(), statestore.StateDir(), version).SaveLesson, PrintJSON: printJSON})
}
