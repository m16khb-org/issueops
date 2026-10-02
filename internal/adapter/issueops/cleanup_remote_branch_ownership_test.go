package issueops

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
)

func TestCleanupRemoteBranchExcludesOtherExecutorsAndWriters(t *testing.T) {
	root, record := remoteBranchTestRecord(t)
	git := remoteBranchGit()
	deps := remoteBranchDeps(git)
	preview, err := CleanupRemoteBranch(context.Background(), root, remoteBranchRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	deps.Git = func(ctx context.Context, repo string, args ...string) (int, string) {
		if args[0] == "push" {
			if _, err := writeIssueOps(context.Background(), root, record); err == nil {
				t.Error("ordinary writer entered during remote deletion")
			}
			observed := false
			second := remoteBranchDeps(git)
			second.VerifyMergedArtifact = func(model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
				observed = true
				return model.CleanupRemoteBranchArtifactHead{HeadRefName: record.Branch, HeadRefOID: remoteBranchTestHeadOID}, nil
			}
			if _, err := CleanupRemoteBranch(ctx, root, remoteBranchRequest(record.ID, false, ""), second); err == nil || observed {
				t.Errorf("second remote executor observed active cycle: observed=%v err=%v", observed, err)
			}
			finish := finishExecutorForTests(root, finishDeps(&fakeFinishGit{branchOID: "abc123"}))
			finish.Observe = func(_ context.Context, _ model.IssueOpsRecord, req model.CleanupFinishRequest) (model.CleanupFinishRequest, error) {
				observed = true
				return req, nil
			}
			observed = false
			if _, err := finish.Run(ctx, finishRequest(record.ID, false, "")); err == nil || observed {
				t.Errorf("finish observed active remote deletion: observed=%v err=%v", observed, err)
			}
		}
		return git.run(ctx, repo, args...)
	}
	if got, err := CleanupRemoteBranch(context.Background(), root, remoteBranchRequest(record.ID, true, preview.Fingerprint), deps); err != nil || !got.Deleted {
		t.Fatalf("owner failed: %+v %v", got, err)
	}
}

func TestCleanupRemoteBranchPreservesRecordChangedDuringDelete(t *testing.T) {
	root, record := remoteBranchTestRecord(t)
	git := remoteBranchGit()
	deps := remoteBranchDeps(git)
	var replacement []byte
	deps.Git = func(ctx context.Context, repo string, args ...string) (int, string) {
		code, out := git.run(ctx, repo, args...)
		if args[0] != "push" {
			return code, out
		}
		current, err := ReadIssueOps(root, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		current.RemoteArtifact.URL = "https://github.com/acme/repo/pull/999"
		_, replacement, err = encodeIssueOpsRecord(current)
		if err != nil {
			t.Fatal(err)
		}
		db, err := sqlstore.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Put(issueOpsBucket, record.ID, replacement); err != nil {
			t.Fatal(err)
		}
		return code, out
	}
	preview, err := CleanupRemoteBranch(context.Background(), root, remoteBranchRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	got, err := CleanupRemoteBranch(context.Background(), root, remoteBranchRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || got.OK || !got.Deleted {
		t.Errorf("changed record received stale finalization: result=%+v err=%v", got, err)
	}
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	raw, found, err := db.Get(issueOpsBucket, record.ID)
	if err != nil || !found || !bytes.Equal(raw, replacement) {
		t.Fatalf("replacement changed: found=%v err=%v", found, err)
	}
}

func TestCleanupRemoteBranchRecoversOnlyItsOwnCrashedAttempt(t *testing.T) {
	for _, absent := range []bool{false, true} {
		for _, operation := range []model.CleanupOperation{model.CleanupOperationRemoteBranch, model.CleanupOperationFinish} {
			t.Run(fmt.Sprintf("%s/absent=%v", operation, absent), func(t *testing.T) {
				root, record := remoteBranchTestRecord(t)
				record.CleanupAttempt = &model.IssueOpsCleanupAttempt{Operation: operation, Token: strings.Repeat("a", 64), StartedAt: "2026-01-01T00:00:00Z"}
				_, raw, err := encodeIssueOpsRecord(record)
				if err != nil {
					t.Fatal(err)
				}
				db, err := sqlstore.Open(root)
				if err != nil {
					t.Fatal(err)
				}
				if err := db.Put(issueOpsBucket, record.ID, raw); err != nil {
					t.Fatal(err)
				}
				git := remoteBranchGit()
				if absent {
					git.remoteOID = ""
				}
				deps := remoteBranchDeps(git)
				observed := false
				verify := deps.VerifyMergedArtifact
				deps.VerifyMergedArtifact = func(a model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
					observed = true
					return verify(a)
				}
				preview, err := CleanupRemoteBranch(context.Background(), root, remoteBranchRequest(record.ID, false, ""), deps)
				current, _, readErr := db.Get(issueOpsBucket, record.ID)
				if readErr != nil || !bytes.Equal(current, raw) {
					t.Fatal("preview rewrote crashed attempt")
				}
				if operation == model.CleanupOperationFinish {
					if err == nil || observed || git.pushes != 0 {
						t.Fatalf("foreign attempt observed or cleared: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				req := remoteBranchRequest(record.ID, true, preview.Fingerprint)
				if absent {
					req.Confirm = false
					req.Fingerprint = "stale"
					if !strings.Contains(preview.NextCommand, "--apply") {
						t.Fatal("no crashed-absence recovery command")
					}
				}
				result, err := CleanupRemoteBranch(context.Background(), root, req, deps)
				if err != nil || !result.OK || result.AlreadyAbsent != absent || result.Deleted == absent {
					t.Fatalf("recovery: %+v %v", result, err)
				}
				currentRecord, err := ReadIssueOps(root, record.ID)
				if err != nil || currentRecord.CleanupAttempt != nil {
					t.Fatalf("attempt not released: %v", err)
				}
				if absent && git.pushes != 0 {
					t.Fatal("absence recovery pushed")
				}
			})
		}
	}
}

func TestCleanupRemoteBranchCancellationReleasesOnlyDrainedAttempt(t *testing.T) {
	root, record := remoteBranchTestRecord(t)
	git := remoteBranchGit()
	deps := remoteBranchDeps(git)
	preview, err := CleanupRemoteBranch(context.Background(), root, remoteBranchRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps.Git = func(ctx context.Context, repo string, args ...string) (int, string) {
		if args[0] == "push" {
			cancel()
			return 1, "cancelled"
		}
		return git.run(ctx, repo, args...)
	}
	got, err := CleanupRemoteBranch(ctx, root, remoteBranchRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || got.Deleted || got.FailedStep != "remote_branch_delete" {
		t.Fatalf("cancellation=%+v %v", got, err)
	}
	current, err := ReadIssueOps(root, record.ID)
	if err != nil || current.CleanupAttempt != nil {
		t.Fatalf("cancelled drained attempt was retained: %v", err)
	}
}

func TestCleanupFinishRejectsRemoteAttemptBeforeObservation(t *testing.T) {
	root, record := remoteBranchTestRecord(t)
	store := CleanupRecordStore{StateRoot: root}
	snapshot, err := store.Load(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	attempt := model.IssueOpsCleanupAttempt{Operation: model.CleanupOperationRemoteBranch, Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}
	armed, err := store.Arm(context.Background(), snapshot, attempt)
	if err != nil {
		t.Fatal(err)
	}
	executor := finishExecutorForTests(root, finishDeps(&fakeFinishGit{branchOID: "abc123"}))
	observed := false
	executor.Observe = func(_ context.Context, _ model.IssueOpsRecord, r model.CleanupFinishRequest) (model.CleanupFinishRequest, error) {
		observed = true
		return r, nil
	}
	if _, err := executor.Run(context.Background(), finishRequest(record.ID, false, "")); err == nil || observed {
		t.Fatalf("finish observed remote-owned record: %v", err)
	}
	current, err := store.Load(context.Background(), record.ID)
	if err != nil || current.Revision != armed.Revision {
		t.Fatalf("foreign attempt changed: %v", err)
	}
}
