package issueopsapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	preflightadapter "issueops/internal/adapter/preflight"
)

type nextGitFake struct {
	calls  []string
	branch string
	head   string
	fail   map[string]bool
}

func (fake *nextGitFake) run(root string, args ...string) (int, string, string) {
	command := strings.Join(args, " ")
	fake.calls = append(fake.calls, root+"|"+command)
	if fake.fail[command] {
		return 128, "", "fatal"
	}
	switch command {
	case "rev-parse --show-toplevel":
		return 0, root + "\n", ""
	case "branch --show-current":
		return 0, " " + fake.branch + "\n", ""
	case "rev-parse HEAD":
		return 0, fake.head + "\n", ""
	}
	return 1, "", "unexpected " + command
}

func (fake *nextGitFake) count(root, command string) int {
	count := 0
	for _, call := range fake.calls {
		if call == root+"|"+command {
			count++
		}
	}
	return count
}

func TestNextGitObservationSharesBranchForTheSameExactRoot(t *testing.T) {
	fake := &nextGitFake{branch: "feature", head: "abc123"}
	observation := newNextGitObservation(fake.run)

	branch := observation.currentBranch("/wt")
	present, stateBranch, head := observation.worktreeState("/wt")

	if branch != "feature" || !present || stateBranch != "feature" || head != "abc123" {
		t.Fatalf("branch=%q present=%v stateBranch=%q head=%q", branch, present, stateBranch, head)
	}
	if got := fake.count("/wt", "branch --show-current"); got != 1 {
		t.Fatalf("branch observations = %d, want 1; calls=%v", got, fake.calls)
	}
	if fake.count("/wt", "rev-parse --show-toplevel") != 1 || fake.count("/wt", "rev-parse HEAD") != 1 || len(fake.calls) != 3 {
		t.Fatalf("worktree probes must stay fresh and unique: %v", fake.calls)
	}
}

func TestNextGitObservationDoesNotShareAcrossRootStrings(t *testing.T) {
	fake := &nextGitFake{branch: "feature", head: "abc123"}
	observation := newNextGitObservation(fake.run)

	observation.currentBranch("/wt")
	observation.worktreeState("/wt/.")

	if fake.count("/wt", "branch --show-current") != 1 || fake.count("/wt/.", "branch --show-current") != 1 {
		t.Fatalf("different root strings must each be observed: %v", fake.calls)
	}
}

func TestNextGitObservationKeepsFailureAndEmptyRootSemantics(t *testing.T) {
	fake := &nextGitFake{branch: "feature", head: "abc123", fail: map[string]bool{"branch --show-current": true}}
	observation := newNextGitObservation(fake.run)

	if branch := observation.currentBranch("/wt"); branch != "" {
		t.Fatalf("failed branch read must be empty, got %q", branch)
	}
	present, branch, head := observation.worktreeState("/wt")
	if !present || branch != "" || head != "abc123" || fake.count("/wt", "branch --show-current") != 1 {
		t.Fatalf("present=%v branch=%q head=%q calls=%v", present, branch, head, fake.calls)
	}

	empty := &nextGitFake{}
	if present, branch, head := newNextGitObservation(empty.run).worktreeState(" "); present || branch != "" || head != "" || len(empty.calls) != 0 {
		t.Fatalf("blank root must not run git: %v", empty.calls)
	}

	missing := &nextGitFake{fail: map[string]bool{"rev-parse --show-toplevel": true}}
	if present, branch, head := newNextGitObservation(missing.run).worktreeState("/gone"); present || branch != "" || head != "" || len(missing.calls) != 1 {
		t.Fatalf("non-worktree must stop after the toplevel probe: %v", missing.calls)
	}
}

func TestNextGitObservationNewRequestObservesExternalChange(t *testing.T) {
	repo := t.TempDir()
	gitOK := func(args ...string) {
		t.Helper()
		full := append([]string{"-c", "user.name=Test", "-c", "user.email=test@example.invalid"}, args...)
		if code, _, stderr := preflightadapter.GitCmd(repo, full...); code != 0 {
			t.Fatalf("git %v: %s", args, stderr)
		}
	}
	commit := func(content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		gitOK("add", "file.txt")
		gitOK("commit", "-q", "-m", content)
	}
	gitOK("init", "-q", "-b", "main")
	commit("one")

	first := newNextGitObservation(preflightadapter.GitCmd)
	branch := first.currentBranch(repo)
	_, _, firstHead := first.worktreeState(repo)

	gitOK("checkout", "-q", "-b", "topic")
	commit("two")

	second := newNextGitObservation(preflightadapter.GitCmd)
	secondBranch := second.currentBranch(repo)
	_, stateBranch, secondHead := second.worktreeState(repo)

	if branch != "main" || secondBranch != "topic" || stateBranch != "topic" {
		t.Fatalf("branches: first=%q second=%q state=%q", branch, secondBranch, stateBranch)
	}
	if firstHead == "" || secondHead == "" || firstHead == secondHead {
		t.Fatalf("a new request must read the new HEAD: first=%q second=%q", firstHead, secondHead)
	}
}
