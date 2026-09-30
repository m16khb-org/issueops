package issueops

import (
	"reflect"
	"testing"
)

func TestGateLedgerFileReadinessDistinguishesPendingEvidence(t *testing.T) {
	missing, warnings := GateLedgerFileReadiness(".issueops/issues/21/gates.md", false, []GateLedgerStatus{
		{ID: "G1", State: "unchecked", Title: "run check"},
		{ID: "G2", State: "evidence_pending", Title: "record proof"},
		{ID: "G3", State: "abandoned", Title: "deferred"},
	})
	if !reflect.DeepEqual(missing, []string{"gates_incomplete:.issueops/issues/21/gates.md"}) ||
		!reflect.DeepEqual(warnings, []string{"gate G1 (unchecked): run check", "gate G2 (evidence_pending): record proof"}) {
		t.Fatalf("missing=%v warnings=%v", missing, warnings)
	}
	if missing, warnings := GateLedgerFileReadiness("gates.md", true, nil); len(missing) != 0 || len(warnings) != 0 {
		t.Fatalf("complete ledger missing=%v warnings=%v", missing, warnings)
	}
}
