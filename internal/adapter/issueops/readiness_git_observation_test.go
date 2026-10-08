package issueops

import (
	"reflect"
	"strings"
	"testing"

	cleanup "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
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
			git, env, _ := NewReadinessGitObservations(run)
			want := ""
			if tc.code == 0 {
				want = strings.TrimSpace(tc.out)
			}
			if out := git.Branch("/repo"); out != want {
				t.Fatalf("Output=%q want=%q", out, want)
			}
			code, out, stderr := env.Git("/repo", "branch", "--show-current")
			if code != tc.code || out != tc.out || stderr != tc.stderr || env.GitOutput("/repo", "branch", "--show-current") != want || calls != 1 {
				t.Fatalf("tuple=(%d,%q,%q) calls=%d", code, out, stderr, calls)
			}
		})
	}
}

func TestGitObservationSharesCleanupWithoutChangingResults(t *testing.T) {
	t.Parallel()

	for _, code := range []int{0, 23} {
		t.Run(string(rune('a'+code)), func(t *testing.T) {
			calls := map[string]int{}
			run := func(_ string, args ...string) (int, string, string) {
				command := strings.Join(args, " ")
				calls[command]++
				switch command {
				case "branch --show-current":
					return 0, " topic\n", ""
				case "remote":
					return 0, "origin\n", ""
				case "status --porcelain=v1":
					return code, "", "status error"
				default:
					return 0, "", ""
				}
			}
			output := func(root string, args ...string) string {
				code, out, _ := run(root, args...)
				if code != 0 {
					return ""
				}
				return strings.TrimSpace(out)
			}
			record := model.IssueOpsRecord{WorktreePath: t.TempDir(), Branch: "topic"}
			want := (cleanup.StructuralStatus{Environment: CleanupStatusEnvironment{RunGit: run, ReadGit: output}}).ForRecord(record, model.IssueOpsCleanupStatusRequest{})
			clear(calls)
			git, env, _ := NewReadinessGitObservations(run)
			got := (cleanup.StructuralStatus{Environment: env}).ForRecord(record, model.IssueOpsCleanupStatusRequest{})
			if !reflect.DeepEqual(got, want) || git.Branch(record.WorktreePath) != "topic" || !git.Clean(record.WorktreePath) {
				t.Fatalf("changed cleanup/output semantics: got=%+v want=%+v", got, want)
			}
			for _, command := range []string{"branch --show-current", "status --porcelain=v1"} {
				if calls[command] != 1 {
					t.Fatalf("%q calls=%d", command, calls[command])
				}
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
	git, env, reset := NewReadinessGitObservations(run)
	for _, root := range []string{"/repo", "/repo/."} {
		git.Branch(root)
		git.Branch(root)
		env.Git(root, "branch", "--show-current", "extra")
		env.Git(root, "branch", "--show-current", "extra")
		if calls[root+" branch --show-current"] != 1 || calls[root+" branch --show-current extra"] != 2 {
			t.Fatalf("root/argv identity lost: %v", calls)
		}
	}
	for range 2 {
		git.Counts("/repo")
		git.Fetch("/repo")
	}
	if git.Counts("/repo") != "after" || git.Branch("/repo") != "after" || calls["/repo fetch --quiet"] != 2 || calls["/repo rev-list --left-right --count HEAD...@{u}"] != 3 || calls["/repo branch --show-current"] != 2 {
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
	git, _, _ := NewReadinessGitObservations(run)
	for range 2 {
		git.BaseAdvanced("/repo", "origin/main")
		git.BaseAdvanced("/repo", "origin/other")
		git.IsWorktree("/repo")
	}
	for command, want := range map[string]int{
		"/repo rev-parse --verify --end-of-options origin/main^{commit}":  2,
		"/repo rev-parse --verify --end-of-options origin/other^{commit}": 2,
		"/repo merge-base --is-ancestor origin/main HEAD":                 2,
		"/repo merge-base --is-ancestor origin/other HEAD":                2,
		"/repo rev-parse --is-inside-work-tree":                           2,
	} {
		if calls[command] != want {
			t.Fatalf("%q calls=%d want=%d: %v", command, calls[command], want, calls)
		}
	}
}
