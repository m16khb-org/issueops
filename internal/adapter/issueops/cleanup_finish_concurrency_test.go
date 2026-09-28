package issueops

import (
	"context"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
)

func TestCleanupFinishExcludesConcurrentApplyAndOrdinaryWriters(t *testing.T) {
	root, record, worktree := finishTestRecord(t, true)
	mutateFinishRecord(t, root, record.ID, func(rec *model.IssueOpsRecord) {
		rec.Execution.Mode = model.ExecutionModeOrca
		rec.Execution.Workspace.Driver = "orca"
		rec.Execution.Orca = &model.OrcaBinding{RuntimeID: "rt", RepoID: "repo", WorktreeID: "wt-1", OwnerHost: "codex", OwnerModel: "m", TaskID: "t", DispatchID: "d"}
	})
	git := &fakeFinishGit{branchOID: "abc123"}
	deps := finishDeps(git)
	deps.OrcaTerminals = readyOrca(t, worktree)
	entered, release := make(chan struct{}), make(chan struct{})
	deps.RemoveOrcaWorktree = func(ctx context.Context, _ string) error {
		close(entered)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	preview, err := CleanupFinish(context.Background(), root, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := CleanupFinish(ctx, root, finishRequest(record.ID, true, preview.Fingerprint), deps)
		finished <- err
	}()
	select {
	case <-entered:
	case err := <-finished:
		t.Fatalf("apply stopped before Orca removal: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	armed, err := ReadIssueOps(root, record.ID)
	if err != nil || armed.CleanupFinishAttempt == nil {
		t.Fatalf("not armed: %v", err)
	}
	for _, apply := range []bool{false, true} {
		if _, err := CleanupFinish(ctx, root, finishRequest(record.ID, apply, preview.Fingerprint), deps); err == nil {
			t.Fatal("second finish entered active executor")
		}
	}
	if _, err := writeIssueOps(root, record); err == nil || !strings.Contains(err.Error(), "cleanup finish") {
		t.Fatalf("ordinary writer bypassed attempt: %v", err)
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if _, err := ReadIssueOps(root, record.ID); err == nil {
		t.Fatal("successful finish retained record")
	}
}

func TestCleanupFinishRecoversCrashedAttemptOnlyAfterExclusiveAcquisition(t *testing.T) {
	root, record, _ := finishTestRecord(t, false)
	record.CleanupFinishAttempt = &model.IssueOpsCleanupFinishAttempt{Token: strings.Repeat("a", 64), StartedAt: "2026-01-01T00:00:00Z"}
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
	lock, err := (FinishLifetimeLock{StateRoot: root}).Acquire(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	git := &fakeFinishGit{branchOID: "abc123"}
	deps := finishDeps(git)
	if _, err := CleanupFinish(context.Background(), root, finishRequest(record.ID, false, ""), deps); err == nil {
		t.Fatal("old timestamp overrode live lifetime lock")
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	preview, err := CleanupFinish(context.Background(), root, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupFinish(context.Background(), root, finishRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil || !result.RecordDeleted {
		t.Fatalf("crash recovery: result=%+v err=%v", result, err)
	}
}
