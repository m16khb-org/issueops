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
		SavePlan: func(result *augmentcontract.PlanResult, key string) error {
			return newSelfWorkflowState(statestore.Dir()).SavePlan(context.Background(), result, key)
		},
		PrintJSON: printJSON,
	})
}

func planSelfAugmentation(req augmentcontract.PlanRequest) augmentcontract.PlanResult {
	return newSelfWorkflowPlanning(issueOpsRoot(), statestore.Dir(), version).Plan(req)
}

func runSelfAugmentLesson(args []string) error {
	return augmentlesson.Run(args, augmentlesson.Deps{Save: func(req augmentcontract.LessonRequest) (augmentcontract.LessonResult, error) {
		return newSelfWorkflowPlanning(issueOpsRoot(), statestore.Dir(), version).SaveLesson(context.Background(), req)
	}, PrintJSON: printJSON})
}
