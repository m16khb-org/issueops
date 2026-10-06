package selfworkflow

import (
	"context"
	"encoding/json"
	statestore "issueops/internal/adapter/outbound/state"
	statecontract "issueops/internal/contract/state"
	"time"

	application "issueops/internal/application/selfverify"
	selfaugmentdomain "issueops/internal/domain/selfaugment"
)

func SaveSelfVerificationSummary(result *SelfAugmentResult, key string) error {
	return application.SaveSummary(result, key, application.SaveSummaryDeps{
		Now:    time.Now,
		Encode: func(snapshot SelfAugmentStateSnapshot) ([]byte, error) { return json.MarshalIndent(snapshot, "", "  ") },
		Write: func(key, content string) (statecontract.StateResult, error) {
			return statestore.NewService().Write(context.Background(), key, content)
		},
		StateDir: statestore.StateDir,
	})
}

func SaveSelfAugmentSummary(result *SelfAugmentResult, key string) error {
	return SaveSelfVerificationSummary(result, key)
}

func NewSelfVerificationSummarySnapshot(result SelfAugmentResult, generatedAt time.Time) SelfAugmentStateSnapshot {
	return selfaugmentdomain.NewSelfVerificationSummarySnapshot(result, generatedAt)
}
