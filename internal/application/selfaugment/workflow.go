package selfaugment

import contract "issueops/internal/contract/selfaugment"

// PlanAndSaveDeps supplies the planning and saving steps PlanAndSave runs.
type PlanAndSaveDeps struct {
	Plan func(req contract.PlanRequest) contract.PlanResult
	Save func(*contract.PlanResult, string) error
}

// PlanAndSave runs the plan and, when save is set, checkpoints it under key. The
// plan is returned even when saving fails, together with the save error.
func PlanAndSave(req contract.PlanRequest, save bool, key string, deps PlanAndSaveDeps) (contract.PlanResult, error) {
	result := deps.Plan(req)
	if save {
		if err := deps.Save(&result, key); err != nil {
			return result, err
		}
	}
	return result, nil
}
