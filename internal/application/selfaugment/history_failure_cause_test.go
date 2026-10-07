package selfaugment

import (
	"encoding/json"
	"strings"
	"testing"

	failure "issueops/internal/contract/failurecause"
	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
)

// TestStoredSummaryComparisonRejectsUnclassifiedFailureEvidence는 비교가 저장된
// 원시 evidence를 다시 정규화하지 않고, 분류 결과와 다른 snapshot을 거부하는지 확인한다.
func TestStoredSummaryComparisonRejectsUnclassifiedFailureEvidence(t *testing.T) {
	raw := strings.Repeat("a", 95) + " b"
	snapshot := contract.SelfAugmentStateSnapshot{SchemaVersion: 1, Kind: "self_verification_summary", Summary: contract.SelfAugmentSummary{FailedSteps: 1, FailureCauseEvidence: []failure.Evidence{{Cause: failure.Model, Code: raw, Source: raw}}}}
	body, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	service := HistoryService{StateDir: func() string { return "/state" }, Read: func(string) (state.StateResult, error) {
		return state.StateResult{Record: state.RecordEnvelope{Content: string(body)}}, nil
	}}
	if _, err := service.Compare("base", "candidate", 20); err == nil || !strings.Contains(err.Error(), "has failure cause") {
		t.Fatalf("unclassified stored evidence must be rejected, got %v", err)
	}
}
