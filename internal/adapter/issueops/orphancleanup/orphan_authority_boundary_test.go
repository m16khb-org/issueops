package orphancleanup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	coreissueops "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	corehealth "issueops/internal/domain/operationalhealth"
)

func TestOrphanApplyPreservesOwnerCreatedDuringMergeObservation(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	fixture := newOrphanCleanupGitFixture(t)
	stateRoot := coreissueops.IssueOpsStateRoot()
	request := fixture.request()
	collect := func(context.Context, string) (corehealth.Snapshot, error) {
		snapshot := fixture.snapshot()
		ids, err := coreissueops.ListIssueOpsIDs(stateRoot)
		if err != nil {
			return snapshot, err
		}
		for _, id := range ids {
			record, err := coreissueops.ReadIssueOpsExisting(stateRoot, id)
			if err != nil {
				return snapshot, err
			}
			snapshot.Cycles = append(snapshot.Cycles, corehealth.Cycle{ID: record.ID, Repo: record.Repo, Branch: record.Branch, WorktreePath: record.WorktreePath, Phase: string(record.Phase)})
		}
		return snapshot, nil
	}
	deps := fixture.deps(collect, nil)
	preview, err := Preview(context.Background(), request, deps)
	if err != nil || !preview.Ready {
		t.Fatalf("initial preview=%+v err=%v", preview, err)
	}
	owner := domain.NewCycleRecord("io-late-owner", fixture.repo, fixture.branch, "2026-09-29T00:00:00Z")
	owner.WorktreePath = fixture.worktree
	deps.VerifyMerged = func(model.IssueOpsRemoteArtifactVerification) error {
		_, err := coreissueops.WriteIssueOps(stateRoot, owner)
		return err
	}
	result, applyErr := Apply(context.Background(), request, ApplyRequest{Confirm: true, Fingerprint: preview.Fingerprint}, deps)
	persisted, err := coreissueops.ReadIssueOpsExisting(stateRoot, owner.ID)
	if err != nil || persisted.WorktreePath != fixture.worktree {
		t.Fatalf("owner did not persist: %+v %v", persisted, err)
	}
	_, statErr := os.Stat(fixture.worktree)
	branch := strings.TrimSpace(gitOutputNoFail(fixture.repo, "rev-parse", "--verify", "refs/heads/"+fixture.branch))
	if applyErr == nil || result.Applied || statErr != nil || branch != preview.HeadSHA {
		t.Fatalf("cleanup deleted newly owned resources: applied=%t error=%v worktree_error=%v branch=%q expected=%q owner=%s", result.Applied, applyErr, statErr, branch, preview.HeadSHA, persisted.ID)
	}
}

func TestOrphanApplyCancellationDuringMergeObservationPreservesResources(t *testing.T) {
	fixture := newOrphanCleanupGitFixture(t)
	request := fixture.request()
	deps := fixture.deps(func(context.Context, string) (corehealth.Snapshot, error) { return fixture.snapshot(), nil }, nil)
	preview, err := Preview(context.Background(), request, deps)
	if err != nil || !preview.Ready {
		t.Fatalf("initial preview=%+v err=%v", preview, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps.VerifyMerged = func(model.IssueOpsRemoteArtifactVerification) error { cancel(); return nil }
	result, applyErr := Apply(ctx, request, ApplyRequest{Confirm: true, Fingerprint: preview.Fingerprint}, deps)
	_, statErr := os.Stat(fixture.worktree)
	branch := strings.TrimSpace(gitOutputNoFail(fixture.repo, "rev-parse", "--verify", "refs/heads/"+fixture.branch))
	if applyErr == nil || result.Applied || statErr != nil || branch != preview.HeadSHA {
		t.Fatalf("cancelled cleanup deleted resources: applied=%t error=%v worktree_error=%v branch=%q expected=%q", result.Applied, applyErr, statErr, branch, preview.HeadSHA)
	}
}

func TestOrphanCleanupPreservesContainedStateStore(t *testing.T) {
	for _, alias := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "symlink ancestor"}[alias], func(t *testing.T) {
			fixture := newOrphanCleanupGitFixture(t)
			stateDir := filepath.Join(fixture.worktree, ".orphan-state")
			if err := os.MkdirAll(stateDir, 0700); err != nil {
				t.Fatal(err)
			}
			if alias {
				link := filepath.Join(t.TempDir(), "state-alias")
				if err := os.Symlink(stateDir, link); err != nil {
					t.Fatal(err)
				}
				stateDir = link
			}
			t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
			exclude := strings.TrimSpace(gitOutput(t, fixture.repo, "rev-parse", "--git-path", "info/exclude"))
			if !filepath.IsAbs(exclude) {
				exclude = filepath.Join(fixture.repo, exclude)
			}
			if err := os.WriteFile(exclude, []byte(".orphan-state/\n"), 0600); err != nil {
				t.Fatal(err)
			}
			stateRoot := coreissueops.IssueOpsStateRoot()
			unrelated := domain.NewCycleRecord("io-unrelated", fixture.repo, "77-another-cycle", "2026-09-29T00:00:00Z")
			if _, err := coreissueops.WriteIssueOps(stateRoot, unrelated); err != nil {
				t.Fatal(err)
			}
			request := fixture.request()
			deps := fixture.deps(func(context.Context, string) (corehealth.Snapshot, error) {
				snapshot := fixture.snapshot()
				snapshot.Cycles = append(snapshot.Cycles, corehealth.Cycle{ID: unrelated.ID, Repo: unrelated.Repo, Branch: unrelated.Branch})
				return snapshot, nil
			}, nil)
			preview, err := Preview(context.Background(), request, deps)
			if err != nil {
				t.Fatal(err)
			}
			if preview.Ready || !contains(preview.Missing, "state_store_outside_target") {
				t.Errorf("contained state preview must refuse specifically: %+v", preview)
			}
			fingerprint := preview.Fingerprint
			if fingerprint == "" {
				fingerprint = "contained-state-must-refuse"
			}
			result, applyErr := Apply(context.Background(), request, ApplyRequest{Confirm: true, Fingerprint: fingerprint}, deps)
			_, stateErr := coreissueops.ReadIssueOpsExisting(stateRoot, unrelated.ID)
			_, worktreeErr := os.Stat(fixture.worktree)
			if result.Applied || applyErr == nil || stateErr != nil || worktreeErr != nil {
				t.Fatalf("cleanup removed its own state/lock namespace: applied=%t apply_error=%v state_error=%v worktree_error=%v", result.Applied, applyErr, stateErr, worktreeErr)
			}
		})
	}
}
