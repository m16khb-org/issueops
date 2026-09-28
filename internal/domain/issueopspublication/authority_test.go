package issueopspublication

import (
	"strings"
	"testing"
)

func TestBeginIntentAuthorityKeepsOperationAndGenerationOrder(t *testing.T) {
	if err := ValidateOperationID("A"); err == nil || !strings.Contains(err.Error(), "32 lowercase hexadecimal") {
		t.Fatalf("invalid operation ID accepted: %v", err)
	}
	facts := BeginAuthorityFacts{Prepared: true, CurrentGeneration: 2, ExpectedGeneration: 2}
	if err := ValidateBeginAuthority(facts); err != nil {
		t.Fatalf("matching authority rejected: %v", err)
	}
	facts.Pending = true
	facts.ExpectedGeneration = 1
	if err := ValidateBeginAuthority(facts); err == nil || !strings.Contains(err.Error(), "authority changed") {
		t.Fatalf("pending intent did not outrank stale generation: %v", err)
	}
	facts.Pending = false
	if err := ValidateBeginAuthority(facts); err == nil || !strings.Contains(err.Error(), "stale lease generation") {
		t.Fatalf("stale generation accepted: %v", err)
	}
}

func TestReceiptAuthoritySeparatesReconcileFromOriginalHolder(t *testing.T) {
	facts := ReceiptAuthorityFacts{
		Prepared: true, Pending: true, PendingOperationID: "op-1", ExpectedOperationID: "op-1",
		Generation: 2, ExpectedGeneration: 2, LeaseStatus: "active", HolderPresent: true,
		HolderHost: "Codex", ExpectedHost: "codex", HolderSessionID: "session", ExpectedSessionID: "session",
		HolderAgentID: "agent", ExpectedAgentID: "agent", CWDMatches: true,
	}
	if err := ValidateReceiptAuthority(facts, true); err != nil {
		t.Fatalf("matching original holder rejected: %v", err)
	}
	facts.Generation = 3
	if err := ValidateReceiptAuthority(facts, true); err == nil || !strings.Contains(err.Error(), "stale execution generation") {
		t.Fatalf("stale original holder accepted: %v", err)
	}
	if err := ValidateReceiptAuthority(facts, false); err != nil {
		t.Fatalf("reconcile required original generation: %v", err)
	}
	facts.PendingOperationID = "other"
	if err := ValidateReceiptAuthority(facts, false); err == nil || !strings.Contains(err.Error(), "external intent changed") {
		t.Fatalf("foreign pending intent accepted: %v", err)
	}
}
