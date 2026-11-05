package policy

import (
	"fmt"
	"testing"
)

func TestDeniedErrorFormatsDefaultAndJoinedReasons(t *testing.T) {
	empty := DeniedError{}
	if got, want := empty.Error(), "command denied by policy"; got != want {
		t.Fatalf("empty DeniedError = %q, want %q", got, want)
	}

	denied := DeniedError{Reasons: []string{"network denied", "shell denied"}}
	if got, want := denied.Error(), "command denied by policy: network denied; shell denied"; got != want {
		t.Fatalf("DeniedError = %q, want %q", got, want)
	}
}

func TestIsPolicyDeniedMatchesDirectErrorOnly(t *testing.T) {
	if !IsPolicyDenied(DeniedError{}) {
		t.Fatal("expected direct DeniedError to match")
	}
	if IsPolicyDenied(fmt.Errorf("wrapped: %w", DeniedError{})) {
		t.Fatal("wrapped DeniedError is not matched by the current direct type assertion contract")
	}
	if IsPolicyDenied(fmt.Errorf("other error")) {
		t.Fatal("non-policy error should not match")
	}
}
