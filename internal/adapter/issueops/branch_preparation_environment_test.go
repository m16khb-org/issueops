package issueops

import (
	"reflect"
	"testing"
)

func TestBranchPreparationEnvironmentsKeepTheirGitRunner(t *testing.T) {
	makeEnvironment := func(repo, oid string) BranchPreparationEnvironment {
		return BranchPreparationEnvironment{RunGit: func(dir string, args ...string) (int, string, string) {
			if dir != repo {
				t.Fatalf("expected repo %s, got %s", repo, dir)
			}
			if reflect.DeepEqual(args, []string{"rev-parse", "--verify", "--end-of-options", "base^{commit}"}) {
				return 0, oid, ""
			}
			if reflect.DeepEqual(args, []string{"remote", "get-url", "origin"}) {
				return 0, "https://github.com/acme/" + oid + ".git", ""
			}
			t.Fatalf("unexpected Git argv: %v", args)
			return 1, "", ""
		}}
	}
	first, second := makeEnvironment("/first", "first"), makeEnvironment("/second", "second")
	for _, tc := range []struct {
		env       BranchPreparationEnvironment
		repo, oid string
	}{{first, "/first", "first"}, {second, "/second", "second"}, {first, "/first", "first"}} {
		oid, err := tc.env.ResolveBaseCommit(tc.repo, " base ")
		if err != nil || oid != tc.oid {
			t.Fatalf("oid=%q err=%v", oid, err)
		}
		key, err := tc.env.ObserveCodeProjectKey(tc.repo, "github")
		if err != nil || key != "github.com/acme/"+tc.oid {
			t.Fatalf("key=%q err=%v", key, err)
		}
	}
}
