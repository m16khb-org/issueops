package stateio

import (
	"context"
	"encoding/json"
	statestore "issueops/internal/adapter/outbound/state"
	statecontract "issueops/internal/contract/state"
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
		Write: func(key, content string) (statecontract.StateResult, error) {
			return statestore.StateWrite(context.Background(), key, content)
		},
		StateDir: statestore.StateDir,
	})
}

func SelfAugmentCandidateIDsByStatus(candidates []SelfAugmentCandidate, status string) []string {
	return domain.CandidateIDsByStatus(candidates, status)
}
