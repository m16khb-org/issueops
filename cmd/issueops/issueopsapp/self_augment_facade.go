package issueopsapp

import (
	"issueops/cmd/issueops/selfworkflow"
	"issueops/cmd/issueops/selfworkflow/augmentcmd"
	statestore "issueops/internal/adapter/outbound/state"
	augmentcontract "issueops/internal/contract/selfaugment"
)

func runSelfAugment(args []string) error {
	selfworkflow.Version = version
	selfworkflow.IssueOpsRoot = issueOpsRoot
	return augmentcmd.Run(args, augmentcmd.Deps{
		RunLesson: runSelfAugmentLesson,
		RunVerify: runSelfVerify,
		Plan:      planSelfAugmentation,
		SavePlan:  newSelfWorkflowState(statestore.StateDir()).SavePlan,
		PrintJSON: printJSON,
	})
}

func planSelfAugmentation(req augmentcontract.SelfAugmentPlanRequest) augmentcontract.SelfAugmentPlanResult {
	selfworkflow.Version = version
	selfworkflow.IssueOpsRoot = issueOpsRoot
	return selfworkflow.PlanSelfAugmentation(req)
}

func runSelfAugmentLesson(args []string) error {
	selfworkflow.Version = version
	selfworkflow.IssueOpsRoot = issueOpsRoot
	return selfworkflow.RunSelfAugmentLesson(args)
}
