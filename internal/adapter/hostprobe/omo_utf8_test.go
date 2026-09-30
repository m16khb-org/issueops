package hostprobe

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestEllipsizeMiddleUTF8Safe(t *testing.T) {
	value := strings.Repeat("가", 40) + "z"
	got := ellipsizeMiddle(value, 64)
	if !utf8.ValidString(got) {
		t.Fatalf("ellipsized value is not valid UTF-8: %q", got)
	}
	if len(got) > 64 {
		t.Fatalf("ellipsized length = %d", len(got))
	}
	head, tail, ok := strings.Cut(got, "...")
	if !ok || !strings.HasPrefix(value, head) || !strings.HasSuffix(value, tail) {
		t.Fatalf("ellipsized value %q does not keep a prefix and a suffix", got)
	}
}
