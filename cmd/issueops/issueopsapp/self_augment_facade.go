package issueopsapp

import (
	"context"
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
		SavePlan: func(result *augmentcontract.SelfAugmentPlanResult, key string) error {
			return newSelfWorkflowState(statestore.StateDir()).SavePlan(context.Background(), result, key)
		},
		PrintJSON: printJSON,
	})
}

func planSelfAugmentation(req augmentcontract.SelfAugmentPlanRequest) augmentcontract.SelfAugmentPlanResult {
	return newSelfWorkflowPlanning(issueOpsRoot(), statestore.StateDir(), version).Plan(req)
}

func runSelfAugmentLesson(args []string) error {
	return augmentlesson.Run(args, augmentlesson.Deps{Save: func(req augmentcontract.SelfAugmentLessonRequest) (augmentcontract.SelfAugmentLessonResult, error) {
		return newSelfWorkflowPlanning(issueOpsRoot(), statestore.StateDir(), version).SaveLesson(context.Background(), req)
	}, PrintJSON: printJSON})
}
