package webfetch

import "testing"

func TestTruncateContentUTF8SafeCountsCharacters(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"가나다라마", 3, "가나다"},
		{"abcdef", 3, "abc"},
		{"가나", 0, "가나"},
		{"가나", 5, "가나"},
	}
	for _, tc := range cases {
		if got := TruncateContent(tc.in, tc.max); got != tc.want {
			t.Fatalf("TruncateContent(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
		}
	}
}
