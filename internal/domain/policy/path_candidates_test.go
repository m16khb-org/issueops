package policy

import "testing"

func TestPolicyPathCandidatesIgnoreRemoteReferences(t *testing.T) {
	for _, arg := range []string{
		"https://github.com/example/repo",
		"ssh://git@github.com/example/repo",
		"git@github.com:example/repo",
	} {
		if got := PathCandidates(arg); len(got) != 0 {
			t.Fatalf("remote reference %q should not be treated as a local path: %+v", arg, got)
		}
	}
	if got := PathCandidates("--file=/tmp/outside"); len(got) != 1 || got[0] != "/tmp/outside" {
		t.Fatalf("flag path candidate not detected: %+v", got)
	}
}
