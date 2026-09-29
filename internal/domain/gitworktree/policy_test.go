package gitworktree

import (
	"strings"
	"testing"
)

func TestCanonicalWorktreeLocationAndIdentityPolicy(t *testing.T) {
	if err := ValidateRequired("io", "/repo", "/repo.worktrees/one", "one", "base"); err != nil {
		t.Fatal(err)
	}
	values := []string{"io", "/repo", "/repo.worktrees/one", "one", "base"}
	for i := range values {
		in := append([]string(nil), values...)
		in[i] = ""
		if err := ValidateRequired(in[0], in[1], in[2], in[3], in[4]); err == nil {
			t.Fatalf("missing field %d accepted", i)
		}
	}
	if err := ValidateLocation("/repo", "/repo.worktrees/one", "/repo.worktrees"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLocation("/repo", "/repo", "/"); err == nil || !strings.Contains(err.Error(), "sibling") {
		t.Fatalf("location precedence: %v", err)
	}
	for _, tc := range []struct {
		same         bool
		branch, head string
	}{{false, "one", "base"}, {true, "other", "base"}, {true, "one", "other"}} {
		if err := ValidateExistingIdentity(tc.same, tc.branch, "one", tc.head, "base"); err == nil {
			t.Fatalf("mismatched identity accepted: %+v", tc)
		}
	}
	if err := ValidateExistingIdentity(true, "one", "one", "base", "base"); err != nil {
		t.Fatal(err)
	}
}
