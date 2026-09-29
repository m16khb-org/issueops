package selfaugment

import (
	"encoding/json"
	"strings"
	"testing"

	failure "issueops/internal/contract/failurecause"
	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
)

func TestStoredSummaryComparisonPreservesFailureEvidenceBoundary(t *testing.T) {
	raw := strings.Repeat("a", 95) + " b"
	snapshot := contract.SelfAugmentStateSnapshot{SchemaVersion: 1, Kind: "self_verification_summary", Summary: contract.SelfAugmentSummary{FailedSteps: 1, FailureCauseEvidence: []failure.Evidence{{Cause: failure.Model, Code: raw, Source: raw}}}}
	body, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	service := HistoryService{StateDir: func() string { return "/state" }, Read: func(string) (state.StateResult, error) {
		return state.StateResult{Record: state.RecordEnvelope{Content: string(body)}}, nil
	}}
	result, err := service.Compare("base", "candidate", 20)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Repeat("a", 95)
	for _, summary := range []contract.SelfAugmentSummary{result.BaselineSummary, result.CandidateSummary} {
		if len(summary.FailureCauseEvidence) != 1 || summary.FailureCauseEvidence[0].Code != want || summary.FailureCauseEvidence[0].Source != want || summary.FailureCauseReason != "model:"+want {
			t.Fatalf("stored summary comparison changed failure normalization: %+v", summary)
		}
	}
}
