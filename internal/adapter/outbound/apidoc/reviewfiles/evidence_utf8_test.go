package reviewfiles

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestThrowDetailUTF8Safe(t *testing.T) {
	got := throwDetail("a" + strings.Repeat("가", 40))
	if !utf8.ValidString(got) {
		t.Fatalf("throw detail is not valid UTF-8: %q", got)
	}
}
