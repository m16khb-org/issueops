package selfworkflow

import (
	"context"
	"encoding/json"
	statestore "issueops/internal/adapter/outbound/state"
	augmentcontract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	"time"

	application "issueops/internal/application/selfverify"
)

func SaveSelfVerificationSummary(result *augmentcontract.SelfAugmentResult, key string) error {
	return application.SaveSummary(result, key, application.SaveSummaryDeps{
		Now: time.Now,
		Encode: func(snapshot augmentcontract.SelfAugmentStateSnapshot) ([]byte, error) {
			return json.MarshalIndent(snapshot, "", "  ")
		},
		Write: func(key, content string) (statecontract.StateResult, error) {
			return statestore.NewService().Write(context.Background(), key, content)
		},
		StateDir: statestore.StateDir,
	})
}
