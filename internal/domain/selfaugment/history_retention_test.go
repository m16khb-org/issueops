package selfaugment

import (
	"reflect"
	"testing"
)

func TestPlanHistoryRetentionKeepsOrderAndRequiresConfirmationToPrune(t *testing.T) {
	keys := []string{"new", "middle", "old"}
	decision := PlanHistoryRetention(keys, 1, true, false)
	if !reflect.DeepEqual(decision.RetainedKeys, []string{"new"}) || !reflect.DeepEqual(decision.CandidateKeys, []string{"middle", "old"}) {
		t.Fatalf("retention keys = %+v", decision)
	}
	if !decision.DryRun || decision.Confirm || decision.Warning != "history_retention_candidates:2" {
		t.Fatalf("unconfirmed prune = %+v", decision)
	}
	if err := ValidateHistoryRetention(0, true, false); err == nil {
		t.Fatal("prune with zero retention limit must fail")
	}
	if err := ValidateHistoryRetention(1, false, true); err == nil {
		t.Fatal("confirmation without prune must fail")
	}
}
