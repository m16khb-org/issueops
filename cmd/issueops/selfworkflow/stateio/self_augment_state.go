package stateio

import (
	"encoding/json"
	"time"

	application "issueops/internal/application/selfaugment"
	domain "issueops/internal/domain/selfaugment"
)

func SaveSelfAugmentPlan(result *SelfAugmentPlanResult, key string) error {
	return application.SavePlan(result, key, application.SavePlanDeps{
		Now: time.Now,
		Encode: func(snapshot SelfAugmentPlanStateSnapshot) ([]byte, error) {
			return json.MarshalIndent(snapshot, "", "  ")
		},
		Write:    StateWrite,
		StateDir: StateDir,
	})
}

func SelfAugmentCandidateIDsByStatus(candidates []SelfAugmentCandidate, status string) []string {
	return domain.CandidateIDsByStatus(candidates, status)
}
