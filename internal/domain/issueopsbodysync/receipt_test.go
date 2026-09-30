package issueopsbodysync

import (
	"strings"
	"testing"
)

func TestBodySyncReceiptRequiresAppliedMatchingReadback(t *testing.T) {
	for _, tc := range []struct {
		name, observed, want string
		applied              bool
	}{
		{"not applied", "digest", "did not report", false},
		{"wrong readback", "other", "readback does not match", true},
		{"verified", "digest", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadback(tc.applied, tc.observed, "digest")
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("receipt error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestBodySyncChildRequiresVerifiedParent(t *testing.T) {
	if err := ValidateChildHierarchy(false, "child", "parent"); err == nil || !strings.Contains(err.Error(), "child is not a provider-native child of parent") {
		t.Fatalf("unverified hierarchy: %v", err)
	}
	if err := ValidateChildHierarchy(true, "child", "parent"); err != nil {
		t.Fatal(err)
	}
}
