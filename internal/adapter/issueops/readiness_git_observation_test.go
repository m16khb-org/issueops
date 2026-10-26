package issueops

import (
	"reflect"
	"strings"
	"testing"
)

func TestGitObservationPreservesTuplesAndOutput(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		code        int
		out, stderr string
	}{
		{"success", 0, "  topic\n", " diagnostic\n"},
		{"detached", 0, "", ""},
		{"failure", 17, " partial output\n", " failure detail\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			run := func(root string, args ...string) (int, string, string) {
				calls++
				if root != "/repo" || !reflect.DeepEqual(args, []string{"branch", "--show-current"}) {
					t.Fatalf("changed invocation: %q %v", root, args)
				}
				return tc.code, tc.out, tc.stderr
			}
			git, _ := NewReadinessGitObservations(run)
			want := ""
			if tc.code == 0 {
				want = strings.TrimSpace(tc.out)
			}
			if out := git.Branch("/repo"); out != want {
				t.Fatalf("Output=%q want=%q", out, want)
			}
			code, out, stderr := git.Run("/repo", "branch", "--show-current")
			if code != tc.code || out != tc.out || stderr != tc.stderr || git.Output("/repo", "branch", "--show-current") != want || calls != 1 {
				t.Fatalf("tuple=(%d,%q,%q) calls=%d", code, out, stderr, calls)
			}
		})
	}
}

func TestGitObservationExactRootsArgumentsAndFetchBoundaries(t *testing.T) {
	t.Parallel()

	calls := map[string]int{}
	phase := "before"
	run := func(root string, args ...string) (int, string, string) {
		calls[root+" "+strings.Join(args, " ")]++
		if args[0] == "fetch" {
			phase = "after"
		}
		return 0, phase, ""
	}
	git, reset := NewReadinessGitObservations(run)
	for _, root := range []string{"/repo", "/repo/."} {
		git.Branch(root)
		git.Branch(root)
		git.Clean(root)
		git.Clean(root)
		git.Run(root, "branch", "--show-current", "extra")
		git.Run(root, "branch", "--show-current", "extra")
		if calls[root+" branch --show-current"] != 1 || calls[root+" status --porcelain=v1"] != 1 || calls[root+" branch --show-current extra"] != 2 {
			t.Fatalf("root/argv identity lost: %v", calls)
		}
	}
	for range 2 {
		git.Counts("/repo")
		git.Fetch("/repo")
	}
	if git.Counts("/repo") != "after" || git.Branch("/repo") != "after" || calls["/repo fetch --quiet --prune"] != 2 || calls["/repo rev-list --left-right --count HEAD...@{u}"] != 3 || calls["/repo branch --show-current"] != 2 {
		t.Fatalf("fetch/post-fetch observations were reused: %v", calls)
	}
	phase = "external"
	reset()
	if git.Branch("/repo") != phase || calls["/repo branch --show-current"] != 3 {
		t.Fatalf("external boundary did not refresh: %v", calls)
	}
}

func TestGitObservationKeepsRefAndHeadProbesFresh(t *testing.T) {
	t.Parallel()

	calls := map[string]int{}
	run := func(root string, args ...string) (int, string, string) {
		calls[root+" "+strings.Join(args, " ")]++
		return 0, "", ""
	}
	git, _ := NewReadinessGitObservations(run)
	for range 2 {
		git.BaseState("/repo", "origin/main")
		git.BaseState("/repo", "origin/other")
		git.IsWorktree("/repo")
	}
	for command, want := range map[string]int{
		"/repo rev-parse --verify --end-of-options origin/main^{commit}":  2,
		"/repo rev-parse --verify --end-of-options origin/other^{commit}": 2,
		"/repo merge-base --is-ancestor origin/main HEAD":                 2,
		"/repo merge-base --is-ancestor origin/other HEAD":                2,
		"/repo rev-parse --show-toplevel":                                 2,
	} {
		if calls[command] != want {
			t.Fatalf("%q calls=%d want=%d: %v", command, calls[command], want, calls)
		}
	}
}
