package issueopsapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/issueops"
	"issueops/internal/adapter/preflight"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// 인계 체인 전체를 production 명령 handler로 한 번에 실행한다:
// prepare -> release -> (인계 관찰) -> replace --preview -> replace --reseed -> claim.
// 단계마다 lease가 generation 1 active -> 1 released -> 2 claimable -> 2 active로
// 바뀌고, 마지막 holder가 claim actor인지 확인한다.
func TestDirectHandoffChainThroughProductionCommands(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	stateRoot := issueOpsStateRoot()
	ctx := context.Background()

	repo := t.TempDir()
	claimWiringGit(t, repo, "init", "-q", "-b", "main")
	claimWiringGit(t, repo, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "--allow-empty", "-q", "-m", "initial")
	baseHead := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "HEAD"))
	record, err := startIssueOpsFixture(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "410-direct-handoff-chain"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/410"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch,
		BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	if _, err := (issueops.CycleRecordStore{StateRoot: stateRoot}).Save(ctx, record); err != nil {
		t.Fatal(err)
	}
	actor := claimWiringActor(t)

	assertLease := func(step string, generation uint64, status issueopscontract.LeaseStatus, wantHolder bool) issueopscontract.WriteLease {
		t.Helper()
		stored, err := issueops.ReadIssueOps(stateRoot, record.ID)
		if err != nil || stored.Execution == nil {
			t.Fatalf("%s: read record: execution=%v err=%v", step, stored.Execution, err)
		}
		lease := stored.Execution.Lease
		if lease.Generation != generation || lease.Status != status || (lease.Holder != nil) != wantHolder {
			t.Fatalf("%s: lease=%+v, want generation %d %s holder=%v", step, lease, generation, status, wantHolder)
		}
		return lease
	}

	prepared, err := productionIssueOpsExecutionDependencies().Prepare(ctx, stateRoot, issueopscontract.ExecutionPrepareRequest{
		ID: record.ID, Actor: actor, CWD: repo, Confirm: true,
	})
	if err != nil || !prepared.OK || prepared.Execution == nil {
		t.Fatalf("prepare: result=%+v err=%v", prepared, err)
	}
	worktree := prepared.Workspace.Root
	if !strings.HasPrefix(worktree, repo+".worktrees"+string(filepath.Separator)) {
		t.Fatalf("prepare did not select the canonical sibling worktree: %q", worktree)
	}
	if _, err := os.Stat(worktree); err != nil {
		t.Fatalf("prepared worktree is missing: %v", err)
	}
	lease := assertLease("prepare", 1, issueopscontract.LeaseStatusActive, true)
	if lease.Holder.SessionID != actor.SessionID {
		t.Fatalf("prepare holder=%+v, want the preparing actor", lease.Holder)
	}

	released, err := issueOpsReleaseHandler(ctx, stateRoot, issueopscontract.ExecutionReleaseRequest{
		ID: record.ID, Generation: 1, Actor: actor, CWD: worktree,
	})
	if err != nil || !released.OK {
		t.Fatalf("release: result=%+v err=%v", released, err)
	}
	assertLease("release", 1, issueopscontract.LeaseStatusReleased, false)

	at := time.Now().UTC().Add(-time.Minute)
	eventNow := handoffDeliveryEventClock(func() time.Time { return at })
	observation := manualHerdrHandoffObservation(record.ID, 1, "chain", at)
	process := *actor.SessionProcess
	observation.Target.Process = &process
	observation.InputAccepted = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherReceipt)
	if _, err := newHandoffDeliveryService(stateRoot).ObserveManual(observation); err != nil {
		t.Fatalf("manual handoff observation on the released generation: %v", err)
	}

	preview, err := newIssueOpsReplacementHandler()(ctx, stateRoot, issueopscontract.ExecutionReplaceRequest{
		ID: record.ID, Action: issueopscontract.ExecutionReplacePreview, ExpectedGeneration: 1, Actor: actor, CWD: worktree,
	}, port.ReplacementInvocation{})
	if err != nil || preview.InventoryFingerprint == "" {
		t.Fatalf("replace --preview: result=%+v err=%v", preview, err)
	}
	assertLease("replace --preview", 1, issueopscontract.LeaseStatusReleased, false)

	reseeded, err := issueOpsReseedHandler(ctx, stateRoot, issueopscontract.ExecutionReseedRequest{
		ID: record.ID, ExpectedGeneration: 1, InventoryFingerprint: preview.InventoryFingerprint,
		Reason: "direct handoff chain", Actor: actor, CWD: worktree, Confirm: true,
	})
	if err != nil || !reseeded.OK || reseeded.ClaimTokenPath == "" {
		t.Fatalf("replace --reseed: result=%+v err=%v", reseeded, err)
	}
	assertLease("replace --reseed", 2, issueopscontract.LeaseStatusClaimable, false)

	claimed, err := issueOpsClaimHandler(ctx, stateRoot, issueopscontract.ExecutionClaimRequest{
		ID: record.ID, Generation: 2, Actor: actor, CWD: worktree, ClaimCurrentToken: true,
	})
	if err != nil || !claimed.OK {
		t.Fatalf("claim --claim-current-token: result=%+v err=%v", claimed, err)
	}
	lease = assertLease("claim", 2, issueopscontract.LeaseStatusActive, true)
	if lease.Holder.Host != actor.Host || lease.Holder.SessionID != actor.SessionID {
		t.Fatalf("claim holder=%+v, want the claim actor %+v", lease.Holder, actor)
	}

	folded, _, err := foldHandoffAuditForTest(stateRoot, observation.LifecycleID, observation.LineageID)
	if err != nil {
		t.Fatal(err)
	}
	got := folded[observation.LifecycleID+"\x00"+observation.LineageID]
	if got.SourceGeneration != 1 || got.OwnerClaimed.Status != issueopscontract.IssueOpsHandoffDeliveryStateObserved ||
		!got.OwnerClaim.Claimed || got.OwnerClaim.Generation != 2 || got.OwnerActor == nil || got.OwnerActor.SessionID != actor.SessionID {
		t.Fatalf("handoff observation after the chain=%+v", got)
	}
}
