package issueops

import (
	"context"
	"reflect"
	"testing"
)

func TestLinkedBranchRemoteRefRequiresSuccessfulExactReadback(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
		out  string
		want string
		fail bool
	}{
		{name: "absent"},
		{name: "present", out: lbSealedBase + "\trefs/heads/" + lbBranch + "\n", want: lbSealedBase},
		{name: "transport", code: 128, fail: true},
		{name: "exit two", code: 2, fail: true},
		{name: "not started", code: -1, fail: true},
		{name: "failed with output", code: 128, out: lbSealedBase + "\trefs/heads/" + lbBranch, fail: true},
		{name: "malformed", out: "not a ref", fail: true},
		{name: "wrong ref", out: lbSealedBase + "\trefs/heads/other", fail: true},
		{name: "duplicate", out: lbSealedBase + "\trefs/heads/" + lbBranch + "\n" + lbSealedBase + "\trefs/heads/" + lbBranch, fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			env := LinkedBranchRemoteRef{RunGit: func(got context.Context, repo string, args ...string) (int, string) {
				if got != ctx || repo != "/repo" || !reflect.DeepEqual(args, []string{"ls-remote", "--heads", "origin", "refs/heads/" + lbBranch}) {
					t.Fatalf("unexpected observation: repo=%q args=%q", repo, args)
				}
				return tc.code, tc.out
			}}
			oid, err := env.Observe(ctx, "/repo", lbBranch)
			if (err != nil) != tc.fail || oid != tc.want {
				t.Fatalf("oid=%q err=%v", oid, err)
			}
		})
	}
}
