package selfworkflow

import (
	"encoding/json"
	statestore "issueops/internal/adapter/outbound/state"
	"time"

	application "issueops/internal/application/selfaugment"
)

func SaveSelfAugmentPlan(result *SelfAugmentPlanResult, key string) error {
	return application.SavePlan(result, key, application.SavePlanDeps{
		Now: time.Now,
		Encode: func(snapshot SelfAugmentPlanStateSnapshot) ([]byte, error) {
			return json.MarshalIndent(snapshot, "", "  ")
		},
		Write:    statestore.StateWrite,
		StateDir: statestore.StateDir,
	})
}
