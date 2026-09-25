package remotecmd

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDurableIssueCreateFailureUTF8Safe(t *testing.T) {
	got := durableIssueCreateFailure(errors.New(strings.Repeat("가", 700)))
	if !utf8.ValidString(got) {
		t.Fatalf("failure diagnostic is not valid UTF-8")
	}
	if len(got) > 2048 {
		t.Fatalf("failure diagnostic length = %d", len(got))
	}
}
