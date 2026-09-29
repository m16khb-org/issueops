package issueops

import (
	"reflect"
	"strings"
	"testing"
)

func TestBranchGitUsesExplicitRunnerAndPreservesObservations(t *testing.T) {
	var calls [][]string
	git := BranchGit{Run: func(root string, args ...string) (int, string, string) {
		if root != "repo" {
			t.Fatal(root)
		}
		calls = append(calls, args)
		return 0, " abc \n", ""
	}}
	if oid, ok := git.RefOID("repo", "refs/heads/branch"); !ok || oid != " abc \n" {
		t.Fatalf("oid=%q ok=%t", oid, ok)
	}
	if found, err := git.OriginPresent("repo", " branch "); err != nil || !found {
		t.Fatalf("found=%t err=%v", found, err)
	}
	want := [][]string{{"rev-parse", "--verify", "--quiet", "refs/heads/branch"}, {"ls-remote", "--heads", "origin", "refs/heads/branch"}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
	git.Run = func(string, ...string) (int, string, string) { return 1, "not-an-observation", " failure \n" }
	if _, ok := git.RefOID("repo", "ref"); ok {
		t.Fatal("failed ref reported present")
	}
	if found, err := git.OriginPresent("repo", "branch"); found || err == nil || !strings.HasSuffix(err.Error(), "failure") {
		t.Fatalf("found=%t err=%v", found, err)
	}
}
