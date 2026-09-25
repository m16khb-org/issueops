package policy

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// mixedWidthText holds characters of width 1, 3, 4, 1, 3 and 2 bytes.
const mixedWidthText = "a가😀b나é"

func TestTruncateBytesUTF8SafeMaximalPrefix(t *testing.T) {
	s := mixedWidthText
	for n := -1; n <= len(s)+1; n++ {
		r := TruncateBytes(s, n)
		if !utf8.ValidString(r) {
			t.Fatalf("TruncateBytes(%q, %d) = %q is not valid UTF-8", s, n, r)
		}
		if len(r) > max(n, 0) {
			t.Fatalf("TruncateBytes(%q, %d) = %q exceeds the limit", s, n, r)
		}
		if !strings.HasPrefix(s, r) {
			t.Fatalf("TruncateBytes(%q, %d) = %q is not a prefix", s, n, r)
		}
		if len(r) < len(s) {
			_, w := utf8.DecodeRuneInString(s[len(r):])
			if len(r)+w <= n {
				t.Fatalf("TruncateBytes(%q, %d) = %q is not maximal", s, n, r)
			}
		}
	}
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"abcdef", 3, "abc"},
		{"가나다", 4, "가"},
		{"가나다", 6, "가나"},
		{"😀a", 3, ""},
		{"😀a", 4, "😀"},
		{"", 5, ""},
		{"abc", 0, ""},
		{"abc", -1, ""},
		{"abc", 10, "abc"},
	}
	for _, tc := range cases {
		if got := TruncateBytes(tc.in, tc.n); got != tc.want {
			t.Fatalf("TruncateBytes(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestTailBytesUTF8SafeMaximalSuffix(t *testing.T) {
	s := mixedWidthText
	for n := -1; n <= len(s)+1; n++ {
		r := TailBytes(s, n)
		if !utf8.ValidString(r) {
			t.Fatalf("TailBytes(%q, %d) = %q is not valid UTF-8", s, n, r)
		}
		if len(r) > max(n, 0) {
			t.Fatalf("TailBytes(%q, %d) = %q exceeds the limit", s, n, r)
		}
		if !strings.HasSuffix(s, r) {
			t.Fatalf("TailBytes(%q, %d) = %q is not a suffix", s, n, r)
		}
		if len(r) < len(s) {
			_, w := utf8.DecodeLastRuneInString(s[:len(s)-len(r)])
			if len(r)+w <= n {
				t.Fatalf("TailBytes(%q, %d) = %q is not maximal", s, n, r)
			}
		}
	}
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"abcdef", 3, "def"},
		{"가나다", 4, "다"},
		{"a😀", 3, ""},
		{"a😀", 4, "😀"},
		{"abc", 0, ""},
		{"abc", 10, "abc"},
	}
	for _, tc := range cases {
		if got := TailBytes(tc.in, tc.n); got != tc.want {
			t.Fatalf("TailBytes(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestTruncateRunesUTF8SafeCountsCharacters(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"가나다라", 2, "가나"},
		{"a😀b", 2, "a😀"},
		{"가나", 5, "가나"},
		{"가나", 0, ""},
		{"가나", -1, ""},
		{"", 3, ""},
	}
	for _, tc := range cases {
		if got := TruncateRunes(tc.in, tc.n); got != tc.want {
			t.Fatalf("TruncateRunes(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestTrimIncompleteRuneUTF8Safe(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"abc", "abc"},
		{"a" + "가", "a" + "가"},
		{"a" + "가"[:1], "a"},
		{"a" + "가"[:2], "a"},
		{"a" + "😀"[:3], "a"},
		{"a" + "😀"[:1], "a"},
		{"a" + "😀", "a" + "😀"},
		{"a\x80", "a\x80"},
	}
	for _, tc := range cases {
		if got := TrimIncompleteRune(tc.in); got != tc.want {
			t.Fatalf("TrimIncompleteRune(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	s := mixedWidthText
	for n := 0; n <= len(s); n++ {
		r := TrimIncompleteRune(s[:n])
		if !utf8.ValidString(r) {
			t.Fatalf("TrimIncompleteRune(%q) = %q is not valid UTF-8", s[:n], r)
		}
		if !strings.HasPrefix(s, r) {
			t.Fatalf("TrimIncompleteRune(%q) = %q is not a prefix", s[:n], r)
		}
		if n-len(r) > utf8.UTFMax-1 {
			t.Fatalf("TrimIncompleteRune(%q) = %q dropped too much", s[:n], r)
		}
	}
}
