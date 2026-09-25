package orphancleanup

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBoundedGitFailureUTF8Safe(t *testing.T) {
	got := boundedGitFailure(strings.Repeat("가", 200))
	if !utf8.ValidString(got) {
		t.Fatalf("git failure is not valid UTF-8: %q", got)
	}
	if len(got) > 512 {
		t.Fatalf("git failure length = %d", len(got))
	}
}
