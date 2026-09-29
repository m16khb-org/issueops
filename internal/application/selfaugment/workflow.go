package selfaugment

import contract "issueops/internal/contract/selfaugment"

type PlanAndSaveDeps struct {
	Plan func(req contract.SelfAugmentPlanRequest) contract.SelfAugmentPlanResult
	Save func(*contract.SelfAugmentPlanResult, string) error
}

func PlanAndSave(req contract.SelfAugmentPlanRequest, save bool, key string, deps PlanAndSaveDeps) (contract.SelfAugmentPlanResult, error) {
	result := deps.Plan(req)
	if save {
		if err := deps.Save(&result, key); err != nil {
			return result, err
		}
	}
	return result, nil
}
