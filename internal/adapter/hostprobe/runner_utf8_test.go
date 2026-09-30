package hostprobe

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBoundedVersionUTF8Safe(t *testing.T) {
	got := boundedVersion(strings.Repeat("가", 100))
	if !utf8.ValidString(got) {
		t.Fatalf("bounded version is not valid UTF-8: %q", got)
	}
	if len(got) > 256 {
		t.Fatalf("bounded version length = %d", len(got))
	}
}
