package remote

import (
	"strings"
	"testing"
)

func TestValidateCommentRefusesWhatMustNotReachTheProvider(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{"  ", "empty"},
		{"waiting for review", "no Korean text"},
		{"리뷰 부탁드립니다\ntoken ghp_abcdefghijklmnopqrstuvwxyz0123456789AB", "line 2"},
		{strings.Repeat("가", 70000), "shorten the body"},
	} {
		if err := ValidateComment("github", tc.body); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("ValidateComment(%.20q) = %v, want %q", tc.body, err, tc.want)
		}
	}
	if err := ValidateComment("gitlab", "리뷰 부탁드립니다."); err != nil {
		t.Fatalf("valid comment refused: %v", err)
	}
}
