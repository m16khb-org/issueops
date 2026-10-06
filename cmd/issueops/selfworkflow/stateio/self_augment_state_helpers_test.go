package stateio

import (
	"context"
	"encoding/json"
	statestore "issueops/internal/adapter/outbound/state"
	augmentcontract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	"time"

	application "issueops/internal/application/selfaugment"
)

func SaveSelfAugmentPlan(result *augmentcontract.SelfAugmentPlanResult, key string) error {
	return application.SavePlan(result, key, application.SavePlanDeps{
		Now: time.Now,
		Encode: func(snapshot augmentcontract.SelfAugmentPlanStateSnapshot) ([]byte, error) {
			return json.MarshalIndent(snapshot, "", "  ")
		},
		Write: func(key, content string) (statecontract.StateResult, error) {
			return statestore.NewService().Write(context.Background(), key, content)
		},
		StateDir: statestore.StateDir,
	})
}
