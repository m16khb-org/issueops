package issueops

import (
	"context"
	"issueops/internal/adapter/preflight"
	"issueops/internal/contract/issueops"
	"issueops/internal/port"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type retryAbandonRemote struct {
	*fakeAbandonRemote
	issueErr error
}

func (p *retryAbandonRemote) CloseIssue(ctx context.Context, req port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error) {
	if p.issueErr != nil {
		return port.IssueProviderCloseIssueResult{}, p.issueErr
	}
	return p.fakeAbandonRemote.CloseIssue(ctx, req)
}
func TestCleanupAbandonRemoteFailureCanRetryUnchangedLocalInventory(t *testing.T) {
	for _, step := range []string{"close_pr", "close_issue", "remote_branch_delete"} {
		t.Run(step, func(t *testing.T) {
			root, record := remoteAbandonRecord(t)
			remote := &retryAbandonRemote{fakeAbandonRemote: &fakeAbandonRemote{artifactBody: port.IssueProviderArtifactBody{State: "OPEN"}, issueBody: port.IssueProviderArtifactBody{State: "OPEN"}, closePR: port.IssueProviderClosePullRequestResult{OK: true, Closed: true, State: "CLOSED"}, closeIssue: port.IssueProviderCloseIssueResult{OK: true, Closed: true, State: "CLOSED"}}}
			git := &remoteAbandonGit{remoteOID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
			switch step {
			case "close_pr":
				remote.closePRErr = context.DeadlineExceeded
			case "close_issue":
				remote.issueErr = context.DeadlineExceeded
			case "remote_branch_delete":
				git.deleteFailed = true
			}
			deps := remoteAbandonDeps(git, remote)
			preview, err := CleanupAbandon(context.Background(), root, remoteAbandonRequest(record.ID, false, ""), deps)
			if err != nil {
				t.Fatal(err)
			}
			got, err := CleanupAbandon(context.Background(), root, remoteAbandonRequest(record.ID, true, preview.Fingerprint), deps)
			if err == nil || got.FailedStep != step || got.RecordDeleted {
				t.Fatalf("did not preserve requested failure: step=%s err=%v", got.FailedStep, err)
			}
			remote.closePRErr = nil
			remote.issueErr = nil
			git.deleteFailed = false
			retry, err := CleanupAbandon(context.Background(), root, remoteAbandonRequest(record.ID, false, ""), deps)
			if err != nil {
				t.Fatalf("published remote failure cannot recover unchanged resources: missing=%v err=%v", retry.Missing, err)
			}
			got, err = CleanupAbandon(context.Background(), root, remoteAbandonRequest(record.ID, true, retry.Fingerprint), deps)
			if err != nil || !got.RecordDeleted {
				t.Fatalf("retry failed: %+v %v", got, err)
			}
		})
	}
}

// Real Git and SQLite exercise failure receipts through preview and apply.
// The worktree-only shape uses a Git observation double because an ordinary
// symbolic HEAD becomes unreadable when its sole branch ref disappears.
func TestCleanupAbandonRemoteFailureRetriesLocalShapes(t *testing.T) {
	for _, step := range []string{"close_pr", "close_issue", "remote_branch_delete"} {
		for _, shape := range []string{"paired", "branch", "worktree"} {
			t.Run(step+"/"+shape, func(t *testing.T) {
				stateRoot, record, worktree := abandonResidueFixture(t)
				mutateFinishRecord(t, stateRoot, record.ID, func(r *issueops.IssueOpsRecord) {
					r.IssueURL = "https://github.com/acme/repo/issues/106"
					r.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/107"}
				})
				if shape == "branch" {
					if code, _, out := preflight.GitCmd(record.Repo, "worktree", "remove", worktree); code != 0 {
						t.Fatal(out)
					}
				}
				remote := &retryAbandonRemote{fakeAbandonRemote: &fakeAbandonRemote{artifactBody: port.IssueProviderArtifactBody{State: "OPEN"}, issueBody: port.IssueProviderArtifactBody{State: "OPEN"}, closePR: port.IssueProviderClosePullRequestResult{OK: true, Closed: true, State: "CLOSED"}, closeIssue: port.IssueProviderCloseIssueResult{OK: true, Closed: true, State: "CLOSED"}}}
				failRemote := true
				switch step {
				case "close_pr":
					remote.closePRErr = context.DeadlineExceeded
				case "close_issue":
					remote.issueErr = context.DeadlineExceeded
				}
				asymmetric := &asymmetricAbandonGit{root: worktree, branch: record.Branch, head: "abc123"}
				deps := CleanupAbandonDeps{Remote: remote, Processes: quietCleanupProcesses(), Git: func(dir string, args ...string) (int, string) {
					switch args[0] {
					case "ls-remote":
						return 0, strings.Repeat("b", 40) + "\trefs/heads/" + record.Branch
					case "push":
						if failRemote && step == "remote_branch_delete" {
							return 1, "remote rejected"
						}
						return 0, ""
					}
					if shape == "worktree" {
						if args[0] == "worktree" && args[1] == "remove" {
							if err := os.RemoveAll(worktree); err != nil {
								return 1, err.Error()
							}
							return 0, ""
						}
						return asymmetric.run(dir, args...)
					}
					code, out, stderr := preflight.GitCmd(dir, args...)
					if code != 0 {
						return code, stderr
					}
					return code, out
				}}
				preview, err := CleanupAbandon(context.Background(), stateRoot, remoteAbandonRequest(record.ID, false, ""), deps)
				if err != nil {
					t.Fatalf("initial preview: %v missing=%v", err, preview.Missing)
				}
				failed, err := CleanupAbandon(context.Background(), stateRoot, remoteAbandonRequest(record.ID, true, preview.Fingerprint), deps)
				if err == nil || failed.FailedStep != step || failed.WorktreeRemoved || failed.BranchDeleted {
					t.Fatalf("expected remote failure before local effects: %+v %v", failed, err)
				}
				remote.closePRErr = nil
				remote.issueErr = nil
				failRemote = false
				// A replacement local resource must fail closed, then the exact original
				// inventory must still be recoverable without editing the failure receipt.
				if shape == "branch" {
					if err := os.MkdirAll(worktree, 0700); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.WriteFile(filepath.Join(worktree, "untracked"), []byte("user work"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				dirtyDeps := deps
				if shape == "worktree" {
					dirtyDeps.Git = func(dir string, args ...string) (int, string) {
						if args[0] == "status" {
							return 0, "?? untracked"
						}
						return deps.Git(dir, args...)
					}
				}
				if got, err := CleanupAbandon(context.Background(), stateRoot, remoteAbandonRequest(record.ID, false, ""), dirtyDeps); err == nil || got.Fingerprint != "" {
					t.Fatalf("changed local state accepted: %+v %v", got, err)
				}
				if shape == "branch" {
					if err := os.Remove(worktree); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.Remove(filepath.Join(worktree, "untracked")); err != nil {
						t.Fatal(err)
					}
				}
				retry, err := CleanupAbandon(context.Background(), stateRoot, remoteAbandonRequest(record.ID, false, ""), deps)
				if err != nil {
					t.Fatalf("unchanged local retry: %v missing=%v", err, retry.Missing)
				}
				applied, err := CleanupAbandon(context.Background(), stateRoot, remoteAbandonRequest(record.ID, true, retry.Fingerprint), deps)
				if err != nil || !applied.RecordDeleted || (shape != "branch" && !applied.WorktreeRemoved) || (shape != "worktree" && !applied.BranchDeleted) {
					t.Fatalf("retry did not finish: %+v %v", applied, err)
				}
			})
		}
	}
}
