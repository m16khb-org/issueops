package orphancleanup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	adapter "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/outbound/processlease"
	"issueops/internal/adapter/outbound/sqlstore"
	app "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	health "issueops/internal/contract/operationalhealth"
	domain "issueops/internal/domain/issueops"
)

type interceptOrphanEnvironment struct {
	app.OrphanEnvironment
	beforeRemove func(context.Context)
	afterRemove  func()
}

func (s interceptOrphanEnvironment) RemoveWorktree(ctx context.Context, repo, path string) error {
	if s.beforeRemove != nil {
		s.beforeRemove(ctx)
	}
	err := s.OrphanEnvironment.RemoveWorktree(ctx, repo, path)
	if err == nil && s.afterRemove != nil {
		s.afterRemove()
	}
	return err
}

func TestOrphanEffectsExcludeNewOwnersAndCompetingCleanup(t *testing.T) {
	fixture := newOrphanCleanupGitFixture(t)
	deps := fixture.deps(func(context.Context, string) (health.Snapshot, error) { return fixture.snapshot(), nil }, nil)
	service := cleaner(deps)
	preview, err := service.Preview(context.Background(), fixture.request())
	if err != nil || !preview.Ready {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	root := adapter.IssueOpsStateRoot()
	owner := domain.NewCycleRecord("io-concurrent-owner", fixture.repo, fixture.branch, "2026-09-29T00:00:00Z")
	owner.WorktreePath = fixture.worktree
	gitRun(t, fixture.repo, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing-remote"))
	observed := false
	environment := service.Environment
	service.Environment = interceptOrphanEnvironment{OrphanEnvironment: environment, beforeRemove: func(ctx context.Context) {
		observed = true
		if _, err := adapter.WriteIssueOps(root, owner); !errors.Is(err, processlease.ErrBusy) {
			t.Errorf("new owner admitted during effect: %v", err)
		}
		lease, err := environment.ExcludeWrites(ctx)
		if lease != nil {
			_ = lease.Close()
		}
		if !errors.Is(err, processlease.ErrBusy) {
			t.Errorf("competing cleanup admitted during effect: %v", err)
		}
	}}
	result, err := service.Apply(context.Background(), fixture.request(), ApplyRequest{Confirm: true, Fingerprint: preview.Fingerprint})
	if err != nil || !result.Applied || !observed {
		t.Fatalf("apply=%+v err=%v observed=%t", result, err, observed)
	}
	ids, err := adapter.ListIssueOpsIDs(root)
	if err != nil || len(ids) != 0 {
		t.Fatalf("cleanup manufactured or admitted owner: %v %v", ids, err)
	}
	if _, err := adapter.WriteIssueOps(root, owner); err != nil {
		t.Fatalf("exclusion leaked after effects: %v", err)
	}
}

func TestOrphanCancellationAfterWorktreeRemovalPreservesBranch(t *testing.T) {
	fixture := newOrphanCleanupGitFixture(t)
	service := cleaner(fixture.deps(func(context.Context, string) (health.Snapshot, error) { return fixture.snapshot(), nil }, nil))
	preview, err := service.Preview(context.Background(), fixture.request())
	if err != nil || !preview.Ready {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Environment = interceptOrphanEnvironment{OrphanEnvironment: service.Environment, afterRemove: cancel}
	result, err := service.Apply(ctx, fixture.request(), ApplyRequest{Confirm: true, Fingerprint: preview.Fingerprint})
	if !errors.Is(err, context.Canceled) || result.Applied || !result.LocalWorktreeRemoved || result.LocalBranchRemoved {
		t.Fatalf("partial result=%+v err=%v", result, err)
	}
	if got := strings.TrimSpace(gitOutputNoFail(fixture.repo, "rev-parse", "--verify", "refs/heads/"+fixture.branch)); got != preview.HeadSHA {
		t.Fatalf("branch lost on cancellation: %q", got)
	}
}

func TestOrphanLocalRefreshRefusesInvalidRecordsAndChangedHead(t *testing.T) {
	for _, kind := range []string{"invalid cycle", "invalid lease index", "changed head"} {
		t.Run(kind, func(t *testing.T) {
			fixture := newOrphanCleanupGitFixture(t)
			deps := fixture.deps(func(context.Context, string) (health.Snapshot, error) { return fixture.snapshot(), nil }, nil)
			preview, err := Preview(context.Background(), fixture.request(), deps)
			if err != nil || !preview.Ready {
				t.Fatalf("preview=%+v err=%v", preview, err)
			}
			deps.VerifyMerged = func(model.IssueOpsRemoteArtifactVerification) error {
				if kind == "changed head" {
					gitRun(t, fixture.worktree, "commit", "--allow-empty", "-m", "late head")
					return nil
				}
				db, err := sqlstore.Open(adapter.IssueOpsStateRoot())
				if err != nil {
					return err
				}
				bucket := "issueops_v1"
				if kind == "invalid lease index" {
					bucket = "lease_holder_v1"
				}
				return db.Put(bucket, "io-corrupt", []byte(`{"schema_version":0}`))
			}
			result, err := Apply(context.Background(), fixture.request(), ApplyRequest{Confirm: true, Fingerprint: preview.Fingerprint}, deps)
			if err == nil || result.Applied || result.LocalWorktreeRemoved || result.LocalBranchRemoved {
				t.Fatalf("unknown/drifted inventory admitted: %+v %v", result, err)
			}
			if _, err := os.Stat(fixture.worktree); err != nil {
				t.Fatalf("worktree lost: %v", err)
			}
			if kind == "changed head" && !strings.Contains(err.Error(), "stale preview fingerprint") {
				t.Fatalf("wrong refusal: %v", err)
			}
			if kind != "changed head" && !contains(result.Missing, "inventory_complete") {
				t.Fatalf("invalid rows not reported: %+v", result)
			}
		})
	}
}
