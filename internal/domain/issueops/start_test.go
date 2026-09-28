package issueops

import (
	"errors"
	"testing"
)

func TestReuseStartRecordDistinguishesResumeCollisionAndUnreadable(t *testing.T) {
	cause := errors.New("corrupt record")
	cases := []struct {
		name    string
		fresh   bool
		readErr error
		missing bool
		reuse   bool
		fails   bool
	}{
		{"resume", false, nil, false, true, false},
		{"collision", true, nil, false, false, true},
		{"new missing", true, cause, true, false, false},
		{"new unreadable", true, cause, false, false, true},
		{"ordinary missing", false, cause, true, false, false},
		{"ordinary read fallback", false, cause, false, false, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			reuse, err := ReuseStartRecord("io-test", tt.fresh, tt.readErr, tt.missing)
			if reuse != tt.reuse || (err != nil) != tt.fails {
				t.Fatalf("reuse=%v err=%v", reuse, err)
			}
			if tt.name == "new unreadable" && !errors.Is(err, cause) {
				t.Fatalf("lost read error: %v", err)
			}
		})
	}
}
