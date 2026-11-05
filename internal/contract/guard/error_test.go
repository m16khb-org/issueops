package guard

import (
	"fmt"
	"testing"
)

func TestBlockedErrorFormatsDefaultAndFirstFindingRule(t *testing.T) {
	empty := BlockedError{}
	if got, want := empty.Error(), "guard check blocked"; got != want {
		t.Fatalf("empty BlockedError = %q, want %q", got, want)
	}

	blocked := BlockedError{Findings: []Finding{
		{Rule: "no-secrets"},
		{Rule: "no-large-file"},
	}}
	if got, want := blocked.Error(), "guard check blocked: no-secrets"; got != want {
		t.Fatalf("BlockedError = %q, want %q", got, want)
	}
}

func TestIsGuardBlockedMatchesDirectErrorOnly(t *testing.T) {
	if !IsGuardBlocked(BlockedError{}) {
		t.Fatal("expected direct BlockedError to match")
	}
	if IsGuardBlocked(fmt.Errorf("wrapped: %w", BlockedError{})) {
		t.Fatal("wrapped BlockedError is not matched by the current direct type assertion contract")
	}
	if IsGuardBlocked(fmt.Errorf("other error")) {
		t.Fatal("non-guard error should not match")
	}
}
