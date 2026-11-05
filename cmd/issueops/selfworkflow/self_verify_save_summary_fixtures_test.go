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

func SaveSelfVerificationSummary(result *augmentcontract.Result, key string) error {
	return application.SaveSummary(result, key, application.SaveSummaryDeps{
		Now: time.Now,
		Encode: func(snapshot augmentcontract.StateSnapshot) ([]byte, error) {
			return json.MarshalIndent(snapshot, "", "  ")
		},
		Write: func(key, content string) (statecontract.Result, error) {
			return statestore.NewService().Write(context.Background(), key, content)
		},
		StateDir: statestore.Dir,
	})
}
