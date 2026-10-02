package preflight

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	preflightcontract "issueops/internal/contract/preflight"
)

type recordingGit struct {
	calls   []string
	outputs map[string]recordedGitResult
}

type recordedGitResult struct {
	code        int
	out, stderr string
}

func (git *recordingGit) run(dir string, args ...string) (int, string, string) {
	command := strings.Join(args, " ")
	git.calls = append(git.calls, dir+"|"+command)
	if result, ok := git.outputs[command]; ok {
		return result.code, result.out, result.stderr
	}
	return 1, "", "unexpected command: " + command
}

func (git *recordingGit) count(prefix string) int {
	count := 0
	for _, call := range git.calls {
		if strings.HasPrefix(strings.SplitN(call, "|", 2)[1], prefix) {
			count++
		}
	}
	return count
}

const historyCommand = "log -10 --format=%h%x00%s%x00%B%x00"

// gitLogRecord renders one commit the way `git log --format=%h%x00%s%x00%B%x00`
// does: three NUL-terminated fields followed by the record newline.
func gitLogRecord(sha, subject, body string) string {
	return sha + "\x00" + subject + "\x00" + body + "\x00\n"
}

func newRecordingGit(history string) *recordingGit {
	return &recordingGit{outputs: map[string]recordedGitResult{
		"rev-parse --show-toplevel":                        {out: "/repo\n"},
		"branch --show-current":                            {out: "topic\n"},
		"rev-parse --short HEAD":                           {out: "abc1234\n"},
		"rev-parse --abbrev-ref --symbolic-full-name @{u}": {code: 128, stderr: "no upstream"},
		"status --porcelain=v1 --branch":                   {out: "## topic\n"},
		"remote -v":                                        {out: ""},
		historyCommand:                                     {out: strings.TrimSpace(history)},
	}}
}

func TestGitPreflightReadsHistoryOnce(t *testing.T) {
	var history strings.Builder
	var wantRecent, wantStyle []preflightcontract.CommitInfo
	for i := 1; i <= 12; i++ {
		sha := fmt.Sprintf("c%02d", i)
		subject := fmt.Sprintf("feat(x): change %d", i)
		body := subject + "\n\nLore:\n- Intent: " + sha + "\n"
		if i == 3 {
			subject = "plain subject\twith tab"
			body = subject + "\n"
		}
		if i <= 10 {
			history.WriteString(gitLogRecord(sha, subject, body))
			wantStyle = append(wantStyle, preflightcontract.CommitInfo{SHA: sha, Subject: subject})
		}
		if i <= 5 {
			wantRecent = append(wantRecent, preflightcontract.CommitInfo{SHA: sha, Subject: subject})
		}
	}
	git := newRecordingGit(history.String())

	facts := GitObserver{Run: func(dir string, args ...string) (int, string, string) {
		return git.run(dir, args...)
	}}.Observe("/repo/sub", "/issueops")

	if got := git.count("log"); got != 1 {
		t.Fatalf("history invocations = %d, want 1; calls=%v", got, git.calls)
	}
	if want := "/repo|" + historyCommand; !containsString(git.calls, want) {
		t.Fatalf("history was not read once at the repository root: %v", git.calls)
	}
	if !facts.GitOK || facts.RepoRoot != "/repo" || facts.Branch != "topic" || facts.Head != "abc1234" {
		t.Fatalf("non-history facts changed: %+v", facts)
	}
	if facts.LastCommit != "c01 feat(x): change 1" {
		t.Fatalf("LastCommit = %q", facts.LastCommit)
	}
	if !reflect.DeepEqual(facts.RecentCommits, wantRecent) {
		t.Fatalf("RecentCommits = %+v, want %+v", facts.RecentCommits, wantRecent)
	}
	if !reflect.DeepEqual(facts.StyleCommits, wantStyle) {
		t.Fatalf("StyleCommits = %+v, want %+v", facts.StyleCommits, wantStyle)
	}
	if len(facts.CommitBodies) != 10 || !strings.HasPrefix(facts.CommitBodies[0], "feat(x): change 1\n\nLore:") {
		t.Fatalf("CommitBodies = %q", facts.CommitBodies)
	}
	if got := facts.CommitBodies[2]; got != "plain subject\twith tab\n" {
		t.Fatalf("body without Lore was altered: %q", got)
	}
}

func TestGitPreflightHistoryHandlesShortEmptyAndMalformedOutput(t *testing.T) {
	for _, tc := range []struct {
		name       string
		result     recordedGitResult
		wantLast   string
		wantRecent []preflightcontract.CommitInfo
		wantBodies []string
	}{
		{
			name:       "single commit with empty body",
			result:     recordedGitResult{out: strings.TrimSpace(gitLogRecord("a1", "only", ""))},
			wantLast:   "a1 only",
			wantRecent: []preflightcontract.CommitInfo{{SHA: "a1", Subject: "only"}},
			wantBodies: []string{""},
		},
		{
			name:       "git log failure keeps the previous empty projection",
			result:     recordedGitResult{code: 128, stderr: "fatal: no commits"},
			wantBodies: []string{""},
		},
		{
			name:       "empty output",
			result:     recordedGitResult{},
			wantBodies: []string{""},
		},
		{
			name:       "truncated trailing tuple is dropped",
			result:     recordedGitResult{out: gitLogRecord("a1", "kept", "kept\n") + "b2\x00half"},
			wantLast:   "a1 kept",
			wantRecent: []preflightcontract.CommitInfo{{SHA: "a1", Subject: "kept"}},
			wantBodies: []string{"kept\n"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			git := newRecordingGit("")
			git.outputs[historyCommand] = tc.result
			facts := GitObserver{Run: git.run}.Observe("/repo", "/issueops")
			if git.count("log") != 1 {
				t.Fatalf("history invocations = %d: %v", git.count("log"), git.calls)
			}
			if facts.LastCommit != tc.wantLast || !reflect.DeepEqual(facts.RecentCommits, tc.wantRecent) || !reflect.DeepEqual(facts.StyleCommits, tc.wantRecent) || !reflect.DeepEqual(facts.CommitBodies, tc.wantBodies) {
				t.Fatalf("projection = last %q recent %+v style %+v bodies %q", facts.LastCommit, facts.RecentCommits, facts.StyleCommits, facts.CommitBodies)
			}
		})
	}
}

func TestGitPreflightNotGitRepositoryDoesNotReadHistory(t *testing.T) {
	git := &recordingGit{outputs: map[string]recordedGitResult{
		"rev-parse --show-toplevel": {code: 128, stderr: "fatal: not a git repository"},
	}}
	facts := GitObserver{Run: git.run}.Observe("/elsewhere", "/issueops")
	if facts.GitOK || facts.ErrorDetail != "fatal: not a git repository" || len(git.calls) != 1 {
		t.Fatalf("facts=%+v calls=%v", facts, git.calls)
	}
}

func TestGitPreflightHistoryProjectionsMatchRealGit(t *testing.T) {
	repo := t.TempDir()
	if code, _, stderr := GitCmd(repo, "init", "-q"); code != 0 {
		t.Fatalf("git init: %s", stderr)
	}
	commit := func(i int) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte(fmt.Sprintf("%d\n", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		if code, _, stderr := GitCmd(repo, "add", "file.txt"); code != 0 {
			t.Fatalf("git add: %s", stderr)
		}
		args := []string{"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-q", "-m", fmt.Sprintf("fix: change %d", i)}
		if i%2 == 0 {
			args = append(args, "-m", fmt.Sprintf("Lore:\n- Intent: %d", i))
		}
		if code, _, stderr := GitCmd(repo, args...); code != 0 {
			t.Fatalf("git commit: %s", stderr)
		}
	}
	for i := 1; i <= 12; i++ {
		commit(i)
	}
	var runs []string
	facts := GitObserver{Run: func(dir string, args ...string) (int, string, string) {
		runs = append(runs, strings.Join(args, " "))
		return GitCmd(dir, args...)
	}}.Observe(repo, "/issueops")

	logs := 0
	for _, run := range runs {
		if strings.HasPrefix(run, "log") {
			logs++
		}
	}
	if logs != 1 {
		t.Fatalf("history invocations = %d: %v", logs, runs)
	}
	if len(facts.RecentCommits) != 5 || len(facts.StyleCommits) != 10 || len(facts.CommitBodies) != 10 {
		t.Fatalf("limits changed: %d/%d/%d", len(facts.RecentCommits), len(facts.StyleCommits), len(facts.CommitBodies))
	}
	if facts.RecentCommits[0].Subject != "fix: change 12" || facts.RecentCommits[4].Subject != "fix: change 8" {
		t.Fatalf("order changed: %+v", facts.RecentCommits)
	}
	if facts.LastCommit != facts.RecentCommits[0].SHA+" fix: change 12" {
		t.Fatalf("LastCommit = %q", facts.LastCommit)
	}
	lore := 0
	for _, body := range facts.CommitBodies {
		if strings.Contains(body, "Lore:") {
			lore++
		}
	}
	if lore != 5 {
		t.Fatalf("lore bodies = %d, want 5 (even changes 3..12)", lore)
	}
}
