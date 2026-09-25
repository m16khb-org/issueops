package issueopspreparation

import (
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationdomain "issueops/internal/domain/issueopspreparation"
)

func TestPlannerGateHintsPreserveRecoveryCommands(t *testing.T) {
	keys := []string{"intent_contract", "design_review", "devils_advocate_review"}
	gates := plannerGateHints(" io-1 ", keys)
	if len(gates) != len(keys) {
		t.Fatalf("gates = %v", gates)
	}
	for i, key := range keys {
		if gates[i].Key != key || !strings.Contains(gates[i].Command, "--id io-1 ") {
			t.Fatalf("gate %d lost its key or command: %+v", i, gates[i])
		}
	}
	if !strings.Contains(gates[2].Command, "--reviewer-context subagent") {
		t.Fatalf("review command lost its required context: %q", gates[2].Command)
	}
}

func TestPlannerEvidenceTreatsMalformedSidecarAsMissing(t *testing.T) {
	record := leasecontract.Record{
		Intent:               []byte(`{"raw_request":"r","interpreted_intent":"i","success_criteria":["c"]}`),
		DesignReview:         []byte(`{"problem_summary":"p","approved":true,"verification":["v"]} broken`),
		DevilsAdvocateReview: []byte(`{"verdict":"pass","recorded_at":"now","reviewed_plan_digest":"abc"}`),
	}
	got := preparationdomain.MissingPlannerGateKeys(plannerEvidence(record))
	if len(got) != 1 || got[0] != "design_review" {
		t.Fatalf("malformed design sidecar = %v", got)
	}
}
