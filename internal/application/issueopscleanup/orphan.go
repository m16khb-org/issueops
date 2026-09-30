package issueopscleanup

import (
	"context"
	"fmt"
	"strings"
	"time"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopsorphancleanup"
	health "issueops/internal/contract/operationalhealth"
	domain "issueops/internal/domain/issueopsorphancleanup"
)

type OrphanEnvironment interface {
	CanonicalPath(string) (string, error)
	StateOutsideTarget(string) (bool, error)
	Clean(context.Context, string) (bool, error)
	CollectLocal(context.Context, string) (health.Snapshot, error)
	ExcludeWrites(context.Context) (CleanupLifetime, error)
	RemoveWorktree(context.Context, string, string) error
	DeleteBranch(context.Context, string, string, string) error
}

type OrphanCleaner struct {
	Environment  OrphanEnvironment
	Collect      func(context.Context, string) (health.Snapshot, error)
	VerifyMerged func(context.Context, model.IssueOpsRemoteArtifactVerification) error
}

func (s OrphanCleaner) normalize(request contract.Request) (contract.Request, error) {
	var err error
	request.RepoRoot, err = s.Environment.CanonicalPath(request.RepoRoot)
	if err != nil {
		return contract.Request{}, fmt.Errorf("orphan cleanup repo root: %w", err)
	}
	request.WorktreePath, err = s.Environment.CanonicalPath(request.WorktreePath)
	if err != nil {
		return contract.Request{}, fmt.Errorf("orphan cleanup worktree path: %w", err)
	}
	return domain.NormalizeRequest(request)
}

func (s OrphanCleaner) Preview(ctx context.Context, request contract.Request) (contract.Result, error) {
	request, err := s.normalize(request)
	if err != nil {
		return contract.Result{}, err
	}
	result := domain.ResultForRequest(request)
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if s.Collect == nil {
		domain.Missing(&result, "inventory_read")
		result.Warnings = append(result.Warnings, "operational inventory reader is unavailable")
		return finishOrphan(result), nil
	}
	snapshot, err := s.Collect(ctx, request.RepoRoot)
	if err != nil {
		domain.Missing(&result, "inventory_read")
		result.Warnings = append(result.Warnings, "operational inventory could not be refreshed")
		return finishOrphan(result), nil
	}
	result = s.localPreview(ctx, request, snapshot)
	domain.MergeEvidence(&result, s.VerifyMerged != nil && s.VerifyMerged(ctx, request.Artifact) == nil)
	if err := ctx.Err(); err != nil {
		return finishOrphan(result), err
	}
	return finishOrphan(result), nil
}

func (s OrphanCleaner) localPreview(ctx context.Context, request contract.Request, snapshot health.Snapshot) contract.Result {
	pathsErr := s.canonicalize(&snapshot)
	clean, cleanErr := s.Environment.Clean(ctx, request.WorktreePath)
	outside, stateErr := s.Environment.StateOutsideTarget(request.WorktreePath)
	return domain.LocalPreview(request, snapshot, domain.LocalObservation{
		PathsObservable: pathsErr == nil, CleanObservable: cleanErr == nil, Clean: clean, StateOutsideTarget: stateErr == nil && outside,
	})
}

func (s OrphanCleaner) canonicalize(snapshot *health.Snapshot) error {
	// Copy slices before path normalization; collectors may retain their snapshot.
	snapshot.GitWorktrees = append([]health.GitWorktree(nil), snapshot.GitWorktrees...)
	snapshot.Cycles = append([]health.Cycle(nil), snapshot.Cycles...)
	snapshot.OrcaWorktrees = append([]health.OrcaWorktree(nil), snapshot.OrcaWorktrees...)
	paths := []*string{&snapshot.RepoRoot}
	for i := range snapshot.GitWorktrees {
		paths = append(paths, &snapshot.GitWorktrees[i].Path)
	}
	for i := range snapshot.Cycles {
		paths = append(paths, &snapshot.Cycles[i].Repo, &snapshot.Cycles[i].WorktreePath)
	}
	for i := range snapshot.OrcaWorktrees {
		paths = append(paths, &snapshot.OrcaWorktrees[i].Path)
	}
	for _, path := range paths {
		if strings.TrimSpace(*path) == "" {
			continue
		}
		value, err := s.Environment.CanonicalPath(*path)
		if err != nil {
			return err
		}
		*path = value
	}
	return nil
}

func finishOrphan(result contract.Result) contract.Result {
	domain.Finish(&result)
	if result.Ready {
		result.Fingerprint = orphanFingerprint(result)
	}
	return result
}

func (s OrphanCleaner) Apply(ctx context.Context, request contract.Request, apply contract.ApplyRequest) (contract.Result, error) {
	if !apply.Confirm {
		return contract.Result{}, fmt.Errorf("orphan cleanup apply requires --confirm")
	}
	if strings.TrimSpace(apply.Fingerprint) == "" {
		return contract.Result{}, fmt.Errorf("orphan cleanup apply requires --fingerprint from a ready preview")
	}
	preview, err := s.Preview(ctx, request)
	preview.Preview = false
	preview.Confirmed = true
	if err != nil {
		return preview, err
	}
	if !preview.Ready {
		return preview, fmt.Errorf("orphan cleanup apply is blocked: %s", strings.Join(preview.Missing, ", "))
	}
	if strings.TrimSpace(apply.Fingerprint) != preview.Fingerprint {
		return preview, fmt.Errorf("stale preview fingerprint: rerun orphan cleanup preview before apply")
	}
	if err := ctx.Err(); err != nil {
		return preview, err
	}
	lifetime, err := s.Environment.ExcludeWrites(ctx)
	if err != nil {
		return preview, err
	}
	defer func() {
		if lifetime != nil {
			_ = lifetime.Close()
		}
	}()
	ctx = lifetime.Context(ctx)
	request, err = s.normalize(request)
	if err != nil {
		return preview, err
	}
	// Only local reads are allowed while all record writers are excluded.
	snapshot, err := s.Environment.CollectLocal(ctx, request.RepoRoot)
	if err != nil {
		return preview, fmt.Errorf("refresh orphan local authority: %w", err)
	}
	current := s.localPreview(ctx, request, snapshot)
	current.RemoteMerged = preview.RemoteMerged
	current = finishOrphan(current)
	current.Preview = false
	current.Confirmed = true
	if !current.Ready {
		return current, fmt.Errorf("orphan cleanup apply is blocked: %s", strings.Join(current.Missing, ", "))
	}
	if current.Fingerprint != preview.Fingerprint {
		return current, fmt.Errorf("stale preview fingerprint: rerun orphan cleanup preview before apply")
	}
	preview = current
	if err := ctx.Err(); err != nil {
		return preview, err
	}
	if err := s.Environment.RemoveWorktree(ctx, preview.RepoRoot, preview.WorktreePath); err != nil {
		return preview, err
	}
	preview.LocalWorktreeRemoved = true
	if err := ctx.Err(); err != nil {
		return preview, err
	}
	if err := s.Environment.DeleteBranch(ctx, preview.RepoRoot, preview.Branch, preview.HeadSHA); err != nil {
		return preview, err
	}
	preview.LocalBranchRemoved = true
	drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	lifetime, err = lifetime.Drain(drainCtx)
	if err != nil {
		return preview, err
	}
	preview.Applied = true
	return preview, nil
}
