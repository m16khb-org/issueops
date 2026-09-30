package gates

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestEvidenceTailUTF8Safe(t *testing.T) {
	got := EvidenceTail(strings.Repeat("가", 100), 200)
	if !utf8.ValidString(got) {
		t.Fatalf("evidence tail is not valid UTF-8: %q", got)
	}
	if len(got) > 200 {
		t.Fatalf("evidence tail length = %d", len(got))
	}
}
