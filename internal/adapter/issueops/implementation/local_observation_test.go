package implementation

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestObserveLocalChangesPreservesChangeSetContracts(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*testing.T, string)
		wantPaths []string
	}{
		{name: "clean", wantPaths: []string{}},
		{
			name: "dirty tracked file",
			mutate: func(t *testing.T, repo string) {
				writeObservationFile(t, repo, "tracked.txt", "dirty\n")
			},
			wantPaths: []string{"tracked.txt"},
		},
		{
			name: "untracked file",
			mutate: func(t *testing.T, repo string) {
				writeObservationFile(t, repo, "untracked.go", "package untracked\n")
			},
			wantPaths: []string{"untracked.go"},
		},
		{
			name: "deleted file",
			mutate: func(t *testing.T, repo string) {
				if err := os.Remove(filepath.Join(repo, "tracked.txt")); err != nil {
					t.Fatal(err)
				}
			},
			wantPaths: []string{"tracked.txt"},
		},
		{
			name: "renamed file",
			mutate: func(t *testing.T, repo string) {
				runGit(t, repo, "mv", "tracked.txt", "renamed.txt")
			},
			wantPaths: []string{"renamed.txt"},
		},
		{
			name: "schema file",
			mutate: func(t *testing.T, repo string) {
				writeObservationFile(t, repo, "db/migrations/001_add_index.sql", "CREATE INDEX idx_x ON x(id);\n")
			},
			wantPaths: []string{"db/migrations/001_add_index.sql"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo, record := newLocalObservationRepo(t)
			if test.mutate != nil {
				test.mutate(t, repo)
			}
			legacyFingerprint := ChangeFingerprint(record)

			observation := ObserveLocalChangesAt(record, repo)

			if !observation.Verified {
				t.Fatalf("stable change set must be verified: %+v", observation)
			}
			if !reflect.DeepEqual(observation.Paths, test.wantPaths) {
				t.Fatalf("paths = %#v, want %#v", observation.Paths, test.wantPaths)
			}
			if observation.Fingerprint != legacyFingerprint {
				t.Fatalf("fingerprint bytes changed: observed=%q legacy=%q", observation.Fingerprint, legacyFingerprint)
			}
			if len(test.wantPaths) == 0 && observation.Fingerprint != "" {
				t.Fatalf("clean change set fingerprint = %q", observation.Fingerprint)
			}
		})
	}
}

func TestObserveLocalChangesUsesFallbackBaseForCommittedDiff(t *testing.T) {
	repo, record := newLocalObservationRepo(t)
	baseSHA := strings.TrimSpace(runGitOutput(t, repo, "rev-parse", "HEAD"))
	runGit(t, repo, "update-ref", "refs/remotes/origin/fallback-base", baseSHA)
	writeObservationFile(t, repo, "only-fallback.go", "package fallback\n")
	runGit(t, repo, "add", "only-fallback.go")
	runGit(t, repo, "commit", "-m", "add fallback-only change")
	record.BranchPrepare.BaseSHA = strings.Repeat("f", 40)
	record.BranchPrepare.BaseBranch = "fallback-base"

	if status := strings.TrimSpace(runGitOutput(t, repo, "status", "--porcelain=v1")); status != "" {
		t.Fatalf("fixture must expose the change only through the fallback diff, status=%q", status)
	}
	observation := ObserveLocalChangesAt(record, repo)

	if !observation.Verified || !reflect.DeepEqual(observation.Paths, []string{"only-fallback.go"}) {
		t.Fatalf("fallback observation = %+v", observation)
	}
	if observation.Fingerprint == "" || observation.Fingerprint != ChangeFingerprint(record) {
		t.Fatalf("fallback fingerprint was not preserved: observation=%q legacy=%q", observation.Fingerprint, ChangeFingerprint(record))
	}
}

func TestObserveLocalChangesPreservesEmptySnapshotWhenAllFallbackRefsFail(t *testing.T) {
	repo, record := newLocalObservationRepo(t)
	writeObservationFile(t, repo, "committed.go", "package committed\n")
	runGit(t, repo, "add", "committed.go")
	runGit(t, repo, "commit", "-m", "add committed change")
	record.BranchPrepare.BaseSHA = strings.Repeat("f", 40)
	record.BranchPrepare.BaseBranch = "missing-base"

	if status := strings.TrimSpace(runGitOutput(t, repo, "status", "--porcelain=v1")); status != "" {
		t.Fatalf("fixture must be clean, status=%q", status)
	}
	observation := ObserveLocalChangesAt(record, repo)

	if !observation.Verified || len(observation.Paths) != 0 || observation.Fingerprint != "" {
		t.Fatalf("failed fallbacks must preserve the legacy empty snapshot: %+v", observation)
	}
	if fingerprint := ChangeFingerprint(record); fingerprint != "" {
		t.Fatalf("legacy fingerprint with no usable base = %q, want empty", fingerprint)
	}
}

func TestObserveLocalChangesDoesNotCrossRepositories(t *testing.T) {
	repoA, recordA := newLocalObservationRepo(t)
	repoB, recordB := newLocalObservationRepo(t)
	writeObservationFile(t, repoA, "only-a.go", "package a\n")
	writeObservationFile(t, repoB, "only-b.go", "package b\n")

	observedA := ObserveLocalChangesAt(recordA, repoA)
	observedB := ObserveLocalChangesAt(recordB, repoB)

	if !observedA.Verified || !reflect.DeepEqual(observedA.Paths, []string{"only-a.go"}) {
		t.Fatalf("repo A observation = %+v", observedA)
	}
	if !observedB.Verified || !reflect.DeepEqual(observedB.Paths, []string{"only-b.go"}) {
		t.Fatalf("repo B observation = %+v", observedB)
	}
}

func TestObserveLocalChangesFailsClosedOnReadFailure(t *testing.T) {
	repo, record := newLocalObservationRepo(t)
	writeObservationFile(t, repo, "unreadable.go", "package unreadable\n")
	wantErr := errors.New("read failed")

	observation := observeLocalChangesAt(record, repo, func(path string) ([]byte, error) {
		if strings.HasSuffix(path, "unreadable.go") {
			return nil, wantErr
		}
		return os.ReadFile(path)
	})

	if observation.Verified || observation.Fingerprint != "" {
		t.Fatalf("unreadable snapshot must not be verified: %+v", observation)
	}
	if !reflect.DeepEqual(observation.Paths, []string{"unreadable.go"}) {
		t.Fatalf("observed paths must remain available for fail-closed schema routing: %#v", observation.Paths)
	}
}

func TestObserveLocalChangesReobservesAConcurrentFileChange(t *testing.T) {
	repo, record := newLocalObservationRepo(t)
	writeObservationFile(t, repo, "tracked.txt", "first\n")
	readCalls := 0

	observation := observeLocalChangesAt(record, repo, func(path string) ([]byte, error) {
		content, err := os.ReadFile(path)
		if err == nil && strings.HasSuffix(path, "tracked.txt") {
			readCalls++
			if readCalls == 1 {
				if writeErr := os.WriteFile(path, []byte("second\n"), 0o600); writeErr != nil {
					t.Fatal(writeErr)
				}
			}
		}
		return content, err
	})

	if !observation.Verified {
		t.Fatalf("one concurrent change must be re-observed to a stable snapshot: %+v", observation)
	}
	stable := ObserveLocalChangesAt(record, repo)
	if observation.Fingerprint != stable.Fingerprint || !reflect.DeepEqual(observation.Paths, stable.Paths) {
		t.Fatalf("retry did not return the final stable snapshot: retry=%+v stable=%+v", observation, stable)
	}
}

func TestObserveLocalChangesRejectsAContinuouslyChangingFile(t *testing.T) {
	repo, record := newLocalObservationRepo(t)
	writeObservationFile(t, repo, "tracked.txt", "changed\n")
	readCalls := 0

	observation := observeLocalChangesAt(record, repo, func(path string) ([]byte, error) {
		if strings.HasSuffix(path, "tracked.txt") {
			readCalls++
			return []byte{byte(readCalls)}, nil
		}
		return os.ReadFile(path)
	})

	if observation.Verified || observation.Fingerprint != "" {
		t.Fatalf("an unstable snapshot must fail closed: %+v", observation)
	}
}

func TestObserveLocalChangesUsesOneBaseResolutionAndTwoSnapshotReads(t *testing.T) {
	repo, record := newLocalObservationRepo(t)
	writeObservationFile(t, repo, "tracked.txt", "dirty\n")
	previousCmd, previousRaw := GitCmd, GitCmdRaw
	var commands []string
	GitCmd = func(dir string, args ...string) (int, string, string) {
		commands = append(commands, strings.Join(args, " "))
		return previousCmd(dir, args...)
	}
	GitCmdRaw = func(dir string, args ...string) (int, string, string) {
		commands = append(commands, strings.Join(args, " "))
		return previousRaw(dir, args...)
	}
	t.Cleanup(func() { GitCmd, GitCmdRaw = previousCmd, previousRaw })

	observation := ObserveLocalChangesAt(record, repo)

	if !observation.Verified {
		t.Fatalf("stable snapshot must verify: %+v", observation)
	}
	counts := map[string]int{}
	for _, command := range commands {
		switch {
		case strings.HasPrefix(command, "rev-parse --verify"):
			counts["base"]++
		case strings.HasPrefix(command, "diff --name-only"):
			counts["diff"]++
		case strings.HasPrefix(command, "status --porcelain"):
			counts["status"]++
		}
	}
	if counts["base"] != 1 || counts["diff"] != 2 || counts["status"] != 2 || len(commands) != 5 {
		t.Fatalf("local snapshot commands = %v (counts=%v), want one base resolution plus two diff/status reads", commands, counts)
	}
}

func newLocalObservationRepo(t *testing.T) (string, model.IssueOpsRecord) {
	t.Helper()
	repo := newImplementationGitRepo(t)
	writeObservationFile(t, repo, "tracked.txt", "base\n")
	runGit(t, repo, "add", "tracked.txt")
	runGit(t, repo, "commit", "-m", "add tracked fixture")
	baseSHA := strings.TrimSpace(runGitOutput(t, repo, "rev-parse", "HEAD"))
	return repo, model.IssueOpsRecord{
		Repo:         repo,
		WorktreePath: repo,
		BranchPrepare: &model.IssueOpsBranchPrepare{
			BaseBranch: "1234-impl",
			BaseSHA:    baseSHA,
		},
	}
}

func writeObservationFile(t *testing.T, repo, rel, content string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
