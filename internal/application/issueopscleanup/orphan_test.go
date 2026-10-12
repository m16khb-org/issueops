package issueopscleanup

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopsorphancleanup"
	health "issueops/internal/contract/operationalhealth"
)

const orphanTestHead = "0123456789abcdef0123456789abcdef01234567"

type orphanTestEnvironment struct {
	events        *[]string
	clean         bool
	cleanErr      error
	collectLocal  health.Snapshot
	excludeErr    error
	removeErr     error
	deleteErr     error
	afterDelete   func()
	drainErr      error
	canonicalFail string
	canonicalSeen []string
	lease         CleanupLifetime
}

func (e *orphanTestEnvironment) CanonicalPath(p string) (string, error) {
	e.canonicalSeen = append(e.canonicalSeen, p)
	if p == e.canonicalFail {
		return "", errors.New("no such path")
	}
	return p, nil
}
func (e *orphanTestEnvironment) StateOutsideTarget(string) (bool, error) { return true, nil }
func (e *orphanTestEnvironment) Clean(context.Context, string) (bool, error) {
	return e.clean, e.cleanErr
}
func (e *orphanTestEnvironment) CollectLocal(context.Context, string) (health.Snapshot, error) {
	*e.events = append(*e.events, "collect-local")
	return e.collectLocal, nil
}
func (e *orphanTestEnvironment) ExcludeWrites(context.Context) (CleanupLifetime, error) {
	*e.events = append(*e.events, "exclude")
	if e.excludeErr != nil {
		return nil, e.excludeErr
	}
	if e.lease != nil {
		return e.lease, nil
	}
	return &finishTestLease{events: e.events, drainErr: e.drainErr}, nil
}
func (e *orphanTestEnvironment) RemoveWorktree(_ context.Context, repo, path string) error {
	*e.events = append(*e.events, "remove "+repo+" "+path)
	return e.removeErr
}
func (e *orphanTestEnvironment) DeleteBranch(_ context.Context, repo, branch, oid string) error {
	*e.events = append(*e.events, "delete-branch "+branch+" "+oid)
	if e.afterDelete != nil {
		e.afterDelete()
	}
	return e.deleteErr
}

func orphanTestSnapshot() health.Snapshot {
	return health.Snapshot{
		RepoRoot: "/repo",
		GitWorktrees: []health.GitWorktree{
			{Path: "/repo", Branch: "main", Head: orphanTestHead, Canonical: true},
			{Path: "/wt/orphan", Branch: "12-orphan", Head: orphanTestHead},
		},
		LocalRefs: []health.GitRef{{Branch: "12-orphan", OID: orphanTestHead, Location: "local"}},
	}
}

func orphanTestRequest() contract.Request {
	return contract.Request{
		ID: "io-orphan", RepoRoot: "/repo", WorktreePath: "/wt/orphan", Branch: "12-orphan",
		Artifact: model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://example.test/pr/1"},
	}
}

func orphanFixture() (OrphanCleaner, *orphanTestEnvironment, *[]string) {
	events := []string{}
	env := &orphanTestEnvironment{events: &events, clean: true, collectLocal: orphanTestSnapshot()}
	return OrphanCleaner{
		Environment:  env,
		Collect:      func(context.Context, string) (health.Snapshot, error) { return orphanTestSnapshot(), nil },
		VerifyMerged: func(context.Context, model.IssueOpsRemoteArtifactVerification) error { return nil },
	}, env, &events
}

func TestOrphanCleanerPreviewIsReadyOnlyWithProviderMergeEvidence(t *testing.T) {
	cleaner, _, _ := orphanFixture()
	preview, err := cleaner.Preview(context.Background(), orphanTestRequest())
	if err != nil || !preview.Ready || !preview.RemoteMerged || preview.Fingerprint == "" || preview.HeadSHA != orphanTestHead {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}

	cleaner.VerifyMerged = func(context.Context, model.IssueOpsRemoteArtifactVerification) error { return errors.New("open") }
	blocked, err := cleaner.Preview(context.Background(), orphanTestRequest())
	if err != nil || blocked.Ready || blocked.Fingerprint != "" || len(blocked.Missing) == 0 {
		t.Fatalf("unverified merge must block: %+v err=%v", blocked, err)
	}
	cleaner.VerifyMerged = nil
	if blocked, _ = cleaner.Preview(context.Background(), orphanTestRequest()); blocked.Ready {
		t.Fatalf("missing verifier must block: %+v", blocked)
	}
}

func TestOrphanCleanerPreviewReportsInventoryReadFailures(t *testing.T) {
	cleaner, _, _ := orphanFixture()
	cleaner.Collect = func(context.Context, string) (health.Snapshot, error) { return health.Snapshot{}, errors.New("boom") }
	preview, err := cleaner.Preview(context.Background(), orphanTestRequest())
	if err != nil || preview.Ready || !reflect.DeepEqual(preview.Missing, []string{"inventory_read"}) {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	cleaner.Collect = nil
	if preview, err = cleaner.Preview(context.Background(), orphanTestRequest()); err != nil || preview.Ready || len(preview.Warnings) == 0 {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
}

func TestOrphanCleanerPreviewRejectsUncanonicalizablePathsAndCancellation(t *testing.T) {
	cleaner, env, _ := orphanFixture()
	env.canonicalFail = "/wt/orphan"
	if _, err := cleaner.Preview(context.Background(), orphanTestRequest()); err == nil || !strings.Contains(err.Error(), "orphan cleanup worktree path") {
		t.Fatalf("err=%v", err)
	}
	env.canonicalFail = ""
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cleaner.Preview(ctx, orphanTestRequest()); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestOrphanCleanerApplyRequiresConfirmAndFingerprintBeforeTouchingAnything(t *testing.T) {
	cleaner, _, events := orphanFixture()
	if _, err := cleaner.Apply(context.Background(), orphanTestRequest(), contract.ApplyRequest{Fingerprint: "x"}); err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("err=%v", err)
	}
	if _, err := cleaner.Apply(context.Background(), orphanTestRequest(), contract.ApplyRequest{Confirm: true, Fingerprint: "  "}); err == nil || !strings.Contains(err.Error(), "--fingerprint") {
		t.Fatalf("err=%v", err)
	}
	if _, err := cleaner.Apply(context.Background(), orphanTestRequest(), contract.ApplyRequest{Confirm: true, Fingerprint: "stale"}); err == nil || !strings.Contains(err.Error(), "stale preview fingerprint") {
		t.Fatalf("err=%v", err)
	}
	if len(*events) != 0 {
		t.Fatalf("effects ran before authorization: %v", *events)
	}
}

func TestOrphanCleanerApplyRemovesWorktreeThenBranchAfterLocalRevalidation(t *testing.T) {
	cleaner, _, events := orphanFixture()
	preview, _ := cleaner.Preview(context.Background(), orphanTestRequest())
	result, err := cleaner.Apply(context.Background(), orphanTestRequest(), contract.ApplyRequest{Confirm: true, Fingerprint: preview.Fingerprint})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Preview || !result.Confirmed || !result.LocalWorktreeRemoved || !result.LocalBranchRemoved || !result.RemoteMerged {
		t.Fatalf("result=%+v", result)
	}
	want := []string{"exclude", "collect-local", "remove /repo /wt/orphan", "delete-branch 12-orphan " + orphanTestHead, "drain", "close"}
	if !reflect.DeepEqual(*events, want) {
		t.Fatalf("events=%v want=%v", *events, want)
	}
}

func TestOrphanCleanerApplyStopsWhenLocalStateDriftsUnderWriteExclusion(t *testing.T) {
	cleaner, env, events := orphanFixture()
	preview, _ := cleaner.Preview(context.Background(), orphanTestRequest())
	// Preview reads were clean; the authoritative local read under write
	// exclusion sees the orphan worktree at a different head.
	drifted := strings.Repeat("a", 40)
	snap := orphanTestSnapshot()
	snap.GitWorktrees[1].Head = drifted
	snap.LocalRefs[0].OID = drifted
	env.collectLocal = snap
	result, err := cleaner.Apply(context.Background(), orphanTestRequest(), contract.ApplyRequest{Confirm: true, Fingerprint: preview.Fingerprint})
	if err == nil || result.Applied || result.LocalWorktreeRemoved {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !contains(*events, "exclude") || !contains(*events, "collect-local") {
		t.Fatalf("revalidation under write exclusion did not run: %v", *events)
	}
	for _, e := range *events {
		if strings.HasPrefix(e, "remove") || strings.HasPrefix(e, "delete-branch") {
			t.Fatalf("destructive effect after drift: %v", *events)
		}
	}
}

func TestOrphanCleanerApplyReportsPartialProgressOnEffectFailure(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name                 string
		setup                func(*orphanTestEnvironment)
		wantWorktree, wantBr bool
		wantCall             bool
	}{
		{"worktree removal fails", func(e *orphanTestEnvironment) { e.removeErr = boom }, false, false, false},
		{"branch deletion fails", func(e *orphanTestEnvironment) { e.deleteErr = boom }, true, false, true},
		{"drain fails", func(e *orphanTestEnvironment) { e.drainErr = boom }, true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleaner, env, events := orphanFixture()
			preview, _ := cleaner.Preview(context.Background(), orphanTestRequest())
			tc.setup(env)
			result, err := cleaner.Apply(context.Background(), orphanTestRequest(), contract.ApplyRequest{Confirm: true, Fingerprint: preview.Fingerprint})
			if !errors.Is(err, boom) || result.Applied || result.LocalWorktreeRemoved != tc.wantWorktree || result.LocalBranchRemoved != tc.wantBr {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if got := contains(*events, "delete-branch 12-orphan "+orphanTestHead); got != tc.wantCall {
				t.Fatalf("branch delete attempted=%v events=%v", got, *events)
			}
		})
	}
}

func TestOrphanCleanerApplyPropagatesWriteExclusionFailure(t *testing.T) {
	cleaner, env, _ := orphanFixture()
	preview, _ := cleaner.Preview(context.Background(), orphanTestRequest())
	env.excludeErr = errors.New("writers active")
	result, err := cleaner.Apply(context.Background(), orphanTestRequest(), contract.ApplyRequest{Confirm: true, Fingerprint: preview.Fingerprint})
	if err == nil || err.Error() != "writers active" || result.Applied {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

type orphanDrainProbe struct {
	remaining time.Duration
	hasLimit  bool
	ctxErr    error
}

func (p *orphanDrainProbe) Context(ctx context.Context) context.Context { return ctx }
func (p *orphanDrainProbe) Close() error                                { return nil }
func (p *orphanDrainProbe) Drain(ctx context.Context) (CleanupLifetime, error) {
	p.ctxErr = ctx.Err()
	if deadline, ok := ctx.Deadline(); ok {
		p.hasLimit, p.remaining = true, time.Until(deadline)
	}
	return p, nil
}

func TestOrphanCleanerApplyGivesDrainAFifteenSecondBudgetDetachedFromCancellation(t *testing.T) {
	cleaner, env, _ := orphanFixture()
	probe := &orphanDrainProbe{}
	env.lease = probe
	preview, _ := cleaner.Preview(context.Background(), orphanTestRequest())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env.afterDelete = cancel
	result, err := cleaner.Apply(ctx, orphanTestRequest(), contract.ApplyRequest{Confirm: true, Fingerprint: preview.Fingerprint})
	if err != nil || !result.Applied {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if ctx.Err() == nil {
		t.Fatal("apply context was never cancelled")
	}
	if !probe.hasLimit || probe.remaining < 10*time.Second || probe.remaining > 15*time.Second {
		t.Fatalf("drain budget hasLimit=%v remaining=%v; want about 15s", probe.hasLimit, probe.remaining)
	}
	if probe.ctxErr != nil {
		t.Fatalf("drain context inherited cancellation: %v", probe.ctxErr)
	}
}

func TestOrphanCleanerPreviewReportsCancellationDuringMergeVerification(t *testing.T) {
	cleaner, _, _ := orphanFixture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cleaner.VerifyMerged = func(context.Context, model.IssueOpsRemoteArtifactVerification) error {
		cancel()
		return nil
	}
	if _, err := cleaner.Preview(ctx, orphanTestRequest()); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v; cancellation after provider verification must surface", err)
	}
}

func TestOrphanCleanerPreviewCanonicalizesEveryRecordedPathAndSkipsBlankOnes(t *testing.T) {
	t.Run("blank inventory paths are never canonicalized", func(t *testing.T) {
		cleaner, env, _ := orphanFixture()
		cleaner.Collect = func(context.Context, string) (health.Snapshot, error) {
			snapshot := orphanTestSnapshot()
			snapshot.RepoRoot = "  "
			return snapshot, nil
		}
		if _, err := cleaner.Preview(context.Background(), orphanTestRequest()); err != nil {
			t.Fatal(err)
		}
		blank, orphan := 0, 0
		for _, seen := range env.canonicalSeen {
			if strings.TrimSpace(seen) == "" {
				blank++
			}
			if seen == "/wt/orphan" {
				orphan++
			}
		}
		if blank != 0 || orphan == 0 {
			t.Fatalf("canonicalized=%q", env.canonicalSeen)
		}
	})
	t.Run("an unresolvable inventory path blocks readiness", func(t *testing.T) {
		cleaner, env, _ := orphanFixture()
		cleaner.Collect = func(context.Context, string) (health.Snapshot, error) {
			snapshot := orphanTestSnapshot()
			snapshot.GitWorktrees = append(snapshot.GitWorktrees, health.GitWorktree{Path: "/wt/gone", Branch: "99-other", Head: orphanTestHead})
			return snapshot, nil
		}
		preview, err := cleaner.Preview(context.Background(), orphanTestRequest())
		if err != nil || !preview.Ready {
			t.Fatalf("baseline preview=%+v err=%v", preview, err)
		}
		env.canonicalFail = "/wt/gone"
		preview, err = cleaner.Preview(context.Background(), orphanTestRequest())
		if err != nil || preview.Ready || preview.Fingerprint != "" {
			t.Fatalf("preview=%+v err=%v", preview, err)
		}
	})
}
