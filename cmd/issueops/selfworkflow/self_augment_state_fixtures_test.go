package selfworkflow

import (
	"context"
	"encoding/json"
	statestore "issueops/internal/adapter/outbound/state"
	statecontract "issueops/internal/contract/state"
	"time"

	application "issueops/internal/application/selfaugment"
)

func SaveSelfAugmentPlan(result *SelfAugmentPlanResult, key string) error {
	return application.SavePlan(result, key, application.SavePlanDeps{
		Now: time.Now,
		Encode: func(snapshot SelfAugmentPlanStateSnapshot) ([]byte, error) {
			return json.MarshalIndent(snapshot, "", "  ")
		},
		Write: func(key, content string) (statecontract.StateResult, error) {
			return statestore.NewService().Write(context.Background(), key, content)
		},
		StateDir: statestore.StateDir,
	})
}
