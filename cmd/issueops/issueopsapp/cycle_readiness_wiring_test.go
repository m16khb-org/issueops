package issueopsapp

import (
	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/preflight"
	cleanup "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestCycleReadinessKeepsCapturedGitCapabilities(t *testing.T) {
	repo := makeGitRepoForContract(t)
	if err := os.WriteFile(filepath.Join(repo, "change.go"), []byte("package fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	readiness := newCycleReadiness()
	t.Chdir(t.TempDir())
	_, changes := readiness.ObserveLocalPR(model.IssueOpsRecord{Repo: repo, WorktreePath: repo})
	if !changes.Verified || changes.Fingerprint == "" {
		t.Fatalf("captured readiness lost actual snapshot: %+v", changes)
	}
}

func TestCycleReadinessSameInstanceMatchesUnsharedResultsAcrossGitChanges(t *testing.T) {
	repo := makeGitRepoForContract(t)
	if err := os.Remove(filepath.Join(repo, ".env")); err != nil {
		t.Fatal(err)
	}
	record := model.IssueOpsRecord{Repo: repo, WorktreePath: repo, Branch: preflight.GitOut(repo, "branch", "--show-current")}
	readiness := newCycleReadiness()
	baseline := readiness
	baseline.NewObservationScope = nil
	baseline.Cleanup = func(record model.IssueOpsRecord) model.IssueOpsCleanupStatus {
		return (cleanup.StructuralStatus{Environment: core.CleanupStatusEnvironment{RunGit: preflight.GitCmd, ReadGit: preflight.GitOut}}).ForRecord(record, model.IssueOpsCleanupStatusRequest{})
	}
	first, _ := readiness.ObserveLocalPR(record)
	want, _ := baseline.ObserveLocalPR(record)
	if !reflect.DeepEqual(first, want) || slices.Contains(first.Missing, "branch_match") || slices.Contains(first.Missing, "worktree_clean") {
		t.Fatalf("initial result changed: got=%+v want=%+v", first, want)
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "-b", "changed-branch"); code != 0 {
		t.Fatal(stderr)
	}
	if err := os.WriteFile(filepath.Join(repo, "untracked.go"), []byte("package fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	second, _ := readiness.ObserveLocalPR(record)
	want, _ = baseline.ObserveLocalPR(record)
	if !reflect.DeepEqual(second, want) || !slices.Contains(second.Missing, "branch_match") || !slices.Contains(second.Missing, "worktree_clean") || !slices.Contains(second.CleanupMissing, "branch_match") || !slices.Contains(second.CleanupMissing, "worktree_clean") {
		t.Fatalf("same readiness instance reused prior Git state: got=%+v want=%+v", second, want)
	}
}

func TestCycleReadinessSharesGitObservationsPerOperation(t *testing.T) {
	repo := makeGitRepoForContract(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	log := filepath.Join(bin, "commands")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$READINESS_GIT_LOG\"\nexec \"$READINESS_REAL_GIT\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("READINESS_REAL_GIT", git)
	t.Setenv("READINESS_GIT_LOG", log)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	readiness := newCycleReadiness()
	record := model.IssueOpsRecord{Repo: repo, WorktreePath: repo}
	for call := 1; call <= 2; call++ {
		readiness.ObserveLocalPR(record)
		data, err := os.ReadFile(log)
		if err != nil {
			t.Fatal(err)
		}
		for _, command := range []string{"branch --show-current", "status --porcelain=v1"} {
			count := 0
			for line := range strings.SplitSeq(string(data), "\n") {
				if line == command {
					count++
				}
			}
			if count != call {
				t.Errorf("operation %d: %q ran %d times, want %d", call, command, count, call)
			}
		}
	}
}
