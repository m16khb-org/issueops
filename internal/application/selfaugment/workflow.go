package selfaugment

import contract "issueops/internal/contract/selfaugment"

type PlanAndSaveDeps struct {
	Plan func(req contract.PlanRequest) contract.PlanResult
	Save func(*contract.PlanResult, string) error
}

func PlanAndSave(req contract.PlanRequest, save bool, key string, deps PlanAndSaveDeps) (contract.PlanResult, error) {
	result := deps.Plan(req)
	if save {
		if err := deps.Save(&result, key); err != nil {
			return result, err
		}
	}
	return result, nil
}
