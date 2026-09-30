package issueopsreview

import (
	"testing"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

func TestSameChangeObservationIdentity(t *testing.T) {
	base := reviewcontract.ChangeObservationIdentity{Root: "/worktree", HasPreparedBase: true, BaseSHA: "abc", BaseBranch: "main"}
	for _, test := range []struct {
		name  string
		other reviewcontract.ChangeObservationIdentity
		want  bool
	}{
		{"equal", base, true},
		{"trimmed", reviewcontract.ChangeObservationIdentity{Root: " /worktree ", HasPreparedBase: true, BaseSHA: " abc ", BaseBranch: " main "}, true},
		{"different root", reviewcontract.ChangeObservationIdentity{Root: "/other", HasPreparedBase: true, BaseSHA: "abc", BaseBranch: "main"}, false},
		{"different base", reviewcontract.ChangeObservationIdentity{Root: "/worktree", HasPreparedBase: true, BaseSHA: "def", BaseBranch: "main"}, false},
		{"missing preparation", reviewcontract.ChangeObservationIdentity{Root: "/worktree"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := SameChangeObservationIdentity(base, test.other); got != test.want {
				t.Fatalf("SameChangeObservationIdentity = %t, want %t", got, test.want)
			}
		})
	}
}
