package issueopsapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestIssueOpsAppClaimWiring(t *testing.T) {
	_, err := issueOpsClaimHandler(context.Background(), t.TempDir(), issueopscontract.ExecutionClaimRequest{
		ID: "io-claim-wiring", Generation: 1, Actor: claimWiringActor(t), CWD: t.TempDir(), ClaimCurrentToken: true,
	})
	if err == nil || !strings.Contains(err.Error(), "issueops record io-claim-wiring not found") {
		t.Fatalf("claim wiring error=%v", err)
	}
}

func TestIssueOpsClaimConsumesCurrentTokenAndRejectsForgedToken(t *testing.T) {
	stateRoot, record, tokenPath := seedDirectClaimFixture(t)
	actor := claimWiringActor(t)
	forged := filepath.Join(t.TempDir(), "forged.token")
	if err := os.WriteFile(forged, []byte("forged-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := issueOpsClaimHandler(context.Background(), stateRoot, issueopscontract.ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: actor, CWD: record.Execution.Workspace.Root, TokenFile: forged,
	})
	if err == nil || result.OK {
		t.Fatalf("forged claim token was accepted: result=%+v err=%v", result, err)
	}
	stored, err := issueops.ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Execution.Lease.Status != issueopscontract.LeaseStatusClaimable || stored.Execution.Lease.Holder != nil {
		t.Fatalf("forged claim changed the lease: %+v", stored.Execution.Lease)
	}

	claimed, err := issueOpsClaimHandler(context.Background(), stateRoot, issueopscontract.ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: actor, CWD: record.Execution.Workspace.Root, TokenFile: tokenPath,
	})
	if err != nil || !claimed.OK {
		t.Fatalf("claim with the issued token: result=%+v err=%v", claimed, err)
	}
	lease := claimed.Execution.Lease
	if lease.Status != issueopscontract.LeaseStatusActive || lease.Generation != 1 || lease.Holder == nil || lease.Holder.SessionID != actor.SessionID {
		t.Fatalf("claimed lease=%+v", lease)
	}
}

func TestSuccessfulDirectClaimAttachesOnlyToExactManualReceiverProcess(t *testing.T) {
	for _, test := range []struct {
		name             string
		sourceGeneration uint64
		claimGeneration  uint64
		manualLineage    bool
		mutateActor      func(*issueopscontract.NativeActor)
		wantClaim        bool
	}{
		{name: "exact process", sourceGeneration: 1, claimGeneration: 2, manualLineage: true, wantClaim: true},
		{name: "pid mismatch", sourceGeneration: 1, claimGeneration: 2, manualLineage: true, mutateActor: func(actor *issueopscontract.NativeActor) {
			actor.SessionProcess.PID++
		}},
		{name: "process start mismatch", sourceGeneration: 1, claimGeneration: 2, manualLineage: true, mutateActor: func(actor *issueopscontract.NativeActor) {
			actor.SessionProcess.StartedAt = time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
		}},
		{name: "executable mismatch", sourceGeneration: 1, claimGeneration: 2, manualLineage: true, mutateActor: func(actor *issueopscontract.NativeActor) {
			actor.SessionProcess.Executable += "-other"
		}},
		{name: "host mismatch", sourceGeneration: 1, claimGeneration: 2, manualLineage: true, mutateActor: func(actor *issueopscontract.NativeActor) {
			actor.Host = "claude"
		}},
		{name: "standard lineage", sourceGeneration: 1, claimGeneration: 2},
		{name: "same generation shortcut", sourceGeneration: 1, claimGeneration: 1, manualLineage: true},
		{name: "stale source generation", sourceGeneration: 1, claimGeneration: 3, manualLineage: true},
		{name: "future source generation", sourceGeneration: 3, claimGeneration: 2, manualLineage: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			const lifecycleID = "io-manual-receiver"
			at := time.Now().UTC().Add(-time.Minute)
			eventNow := handoffDeliveryEventClock(func() time.Time { return at })
			observation := manualHerdrHandoffObservation(lifecycleID, test.sourceGeneration, "claim", at)
			if !test.manualLineage {
				observation.AttemptID = strings.TrimPrefix(observation.AttemptID, "manual-direct:")
				observation.LineageID = strings.TrimPrefix(observation.LineageID, "manual-direct:")
			}
			actor := claimWiringActor(t)
			process := *actor.SessionProcess
			observation.Target.Process = &process
			observation.InputAccepted = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherReceipt)
			if _, err := newHandoffDeliveryAudit(stateRoot).Append(observation); err != nil {
				t.Fatal(err)
			}
			if test.mutateActor != nil {
				test.mutateActor(&actor)
			}
			claimedAt := time.Now().UTC().Format(time.RFC3339Nano)
			result := issueopscontract.ExecutionResult{OK: true, ID: lifecycleID, Execution: issueopscontract.Execution{
				Lease: issueopscontract.WriteLease{Generation: test.claimGeneration, Status: issueopscontract.LeaseStatusActive, Holder: &actor, ClaimedAt: claimedAt},
			}}
			if err := newHandoffDeliveryService(stateRoot).ObserveClaim(result); err != nil {
				t.Fatal(err)
			}
			folded, _, err := foldHandoffAuditForTest(stateRoot, observation.LifecycleID, observation.LineageID)
			if err != nil {
				t.Fatal(err)
			}
			got := folded[observation.LifecycleID+"\x00"+observation.LineageID]
			claimed := got.OwnerClaimed.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved
			if claimed != test.wantClaim {
				t.Fatalf("owner claim attached=%v want=%v observation=%+v", claimed, test.wantClaim, got)
			}
		})
	}
}

func TestSuccessfulDirectClaimUsesCmuxOnlyAfterRawInputAndExactReceiverCorrelation(t *testing.T) {
	for _, test := range []struct {
		name          string
		created       bool
		inputAccepted bool
		process       bool
		wantClaim     bool
	}{
		{name: "staged only"},
		{name: "workspace created only", created: true},
		{name: "raw input without receiver", created: true, inputAccepted: true},
		{name: "raw input exact receiver", created: true, inputAccepted: true, process: true, wantClaim: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			actor := claimWiringActor(t)
			createdAt := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
			observation := manualCmuxHandoffObservation("io-cmux-claim", 1)
			observation.CreatedAt, observation.UpdatedAt = createdAt, createdAt
			observation.CallStaged.ObservedAt = createdAt
			if test.created {
				observation.Target.WorkspaceID = "11111111-1111-4111-8111-111111111111"
				observation.Target.SurfaceID = "22222222-2222-4222-8222-222222222222"
			}
			if test.inputAccepted {
				observation.InputAccepted = issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateObserved, ObservedAt: createdAt, Evidence: issueopscontract.IssueOpsHandoffDeliveryEvidenceRawInput}
			}
			if test.process {
				process := *actor.SessionProcess
				observation.Target.Process = &process
				observation.Target.ProcessIncarnation = fmt.Sprintf("%d:%s:%s", process.PID, process.StartedAt, process.Executable)
			}
			if _, err := newHandoffDeliveryAudit(stateRoot).Append(observation); err != nil {
				t.Fatal(err)
			}
			claimedAt := time.Now().UTC().Format(time.RFC3339Nano)
			result := issueopscontract.ExecutionResult{OK: true, ID: observation.LifecycleID, Execution: issueopscontract.Execution{
				Lease: issueopscontract.WriteLease{Generation: 2, Status: issueopscontract.LeaseStatusActive, Holder: &actor, ClaimedAt: claimedAt},
			}}
			if err := newHandoffDeliveryService(stateRoot).ObserveClaim(result); err != nil {
				t.Fatal(err)
			}
			folded, _, err := foldHandoffAuditForTest(stateRoot, observation.LifecycleID, observation.LineageID)
			if err != nil {
				t.Fatal(err)
			}
			got := folded[observation.LifecycleID+"\x00"+observation.LineageID]
			if claimed := got.OwnerClaimed.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved; claimed != test.wantClaim {
				t.Fatalf("claim=%v want=%v observation=%+v", claimed, test.wantClaim, got)
			}
		})
	}
}

func TestSuccessfulDirectClaimRejectsAmbiguousManualLineages(t *testing.T) {
	stateRoot := t.TempDir()
	const lifecycleID = "io-manual-ambiguous"
	at := time.Now().UTC().Add(-time.Minute)
	eventNow := handoffDeliveryEventClock(func() time.Time { return at })
	actor := claimWiringActor(t)
	for _, suffix := range []string{"first", "second"} {
		observation := manualHerdrHandoffObservation(lifecycleID, 1, suffix, at)
		process := *actor.SessionProcess
		observation.Target.Process = &process
		observation.InputAccepted = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherReceipt)
		if _, err := newHandoffDeliveryAudit(stateRoot).Append(observation); err != nil {
			t.Fatal(err)
		}
	}
	result := issueopscontract.ExecutionResult{OK: true, ID: lifecycleID, Execution: issueopscontract.Execution{
		Lease: issueopscontract.WriteLease{
			Generation: 2, Status: issueopscontract.LeaseStatusActive, Holder: &actor,
			ClaimedAt: time.Now().UTC().Format(time.RFC3339Nano),
		},
	}}
	if err := newHandoffDeliveryService(stateRoot).ObserveClaim(result); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous manual lineages were accepted: %v", err)
	}
}

// 인계 관측은 trace handoff-delivery와 같은 production manual producer(ObserveManual)로
// released generation 1에 남기고, 그 뒤 reseed한 generation 2를 claim해도 관측이
// source generation 1 그대로 owner claim을 이어받는지 확인한다.
func TestSuccessfulDirectClaimObservesReleasedReseededGeneration(t *testing.T) {
	stateRoot, record, oldToken := seedDirectClaimFixture(t)
	record.Execution.Lease.Status = issueopscontract.LeaseStatusReleased
	record.Execution.Lease.Holder = nil
	record.Execution.Lease.ClaimTokenSHA256 = ""
	if err := os.Remove(oldToken); err != nil {
		t.Fatal(err)
	}
	if _, err := (issueops.CycleRecordStore{StateRoot: stateRoot}).Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}

	actor := claimWiringActor(t)
	at := time.Now().UTC().Add(-time.Minute)
	eventNow := handoffDeliveryEventClock(func() time.Time { return at })
	observation := manualHerdrHandoffObservation(record.ID, 1, "reseed-claim", at)
	process := *actor.SessionProcess
	observation.Target.Process = &process
	observation.InputAccepted = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherReceipt)
	if _, err := newHandoffDeliveryService(stateRoot).ObserveManual(observation); err != nil {
		t.Fatalf("production manual producer rejected the released-generation observation: %v", err)
	}

	preview, err := newIssueOpsReplacementHandler()(context.Background(), stateRoot, issueopscontract.ExecutionReplaceRequest{
		ID: record.ID, Action: issueopscontract.ExecutionReplacePreview, ExpectedGeneration: 1, Actor: actor, CWD: record.Execution.Workspace.Root,
	}, port.ReplacementInvocation{})
	if err != nil {
		t.Fatalf("preview released direct reseed: %v", err)
	}
	reseeded, err := issueOpsReseedHandler(context.Background(), stateRoot, issueopscontract.ExecutionReseedRequest{
		ID: record.ID, ExpectedGeneration: 1, InventoryFingerprint: preview.InventoryFingerprint,
		Reason: "manual direct handoff", Actor: actor, CWD: record.Execution.Workspace.Root, Confirm: true,
	})
	if err != nil {
		t.Fatalf("reseed released direct execution: %v", err)
	}
	if reseeded.Execution.Lease.Generation != 2 || reseeded.Execution.Lease.Status != issueopscontract.LeaseStatusClaimable {
		t.Fatalf("reseeded lease=%+v", reseeded.Execution.Lease)
	}

	claimed, err := issueOpsClaimHandler(context.Background(), stateRoot, issueopscontract.ExecutionClaimRequest{
		ID: record.ID, Generation: 2, Actor: actor, CWD: record.Execution.Workspace.Root, ClaimCurrentToken: true,
	})
	if err != nil || !claimed.OK {
		t.Fatalf("claim reseeded direct execution: result=%+v err=%v", claimed, err)
	}
	folded, _, err := foldHandoffAuditForTest(stateRoot, observation.LifecycleID, observation.LineageID)
	if err != nil {
		t.Fatal(err)
	}
	got := folded[observation.LifecycleID+"\x00"+observation.LineageID]
	if got.SourceGeneration != 1 || got.OwnerClaimed.Status != issueopscontract.IssueOpsHandoffDeliveryStateObserved ||
		!got.OwnerClaim.Claimed || got.OwnerClaim.Generation != 2 || got.OwnerActor == nil || got.OwnerActor.SessionID != actor.SessionID {
		t.Fatalf("released-to-reseeded claim observation=%+v", got)
	}
}

func TestIssueOpsClaimDoesNotReturnFalseFailureWhenObservationAuditIsUnsafe(t *testing.T) {
	stateRoot, record, token := seedDirectClaimFixture(t)
	seed := manualHerdrHandoffObservation(record.ID, 1, "unsafe-audit", time.Now().UTC().Add(-time.Minute))
	if _, err := newHandoffDeliveryAudit(stateRoot).Append(seed); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stateRoot, "audit", "handoff-delivery.jsonl")
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := issueOpsClaimHandler(context.Background(), stateRoot, issueopscontract.ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: claimWiringActor(t), CWD: record.Execution.Workspace.Root, TokenFile: token,
	})
	if err != nil || !result.OK || result.Execution.Lease.Status != issueopscontract.LeaseStatusActive {
		t.Fatalf("committed claim was reported as failed: result=%+v err=%v", result, err)
	}
}

func TestIssueOpsConcurrentReceiversHaveOneAuthorityHolderAndLoserDoesNoWork(t *testing.T) {
	stateRoot, record, token := seedDirectClaimFixture(t)
	actors := []issueopscontract.NativeActor{claimWiringActor(t), claimWiringActor(t)}
	actors[0].SessionID = "receiver-a"
	actors[1].SessionID = "receiver-b"
	var ready sync.WaitGroup
	ready.Add(2)
	start := make(chan struct{})
	var successes atomic.Int32
	var work [2]atomic.Int32
	errs := make([]error, 2)
	results := make([]issueopscontract.ExecutionResult, 2)
	var done sync.WaitGroup
	done.Add(2)
	for index := range actors {
		go func(index int) {
			defer done.Done()
			ready.Done()
			<-start
			results[index], errs[index] = issueOpsClaimHandler(context.Background(), stateRoot, issueopscontract.ExecutionClaimRequest{
				ID: record.ID, Generation: 1, Actor: actors[index], CWD: record.Execution.Workspace.Root, TokenFile: token,
			})
			if errs[index] == nil && results[index].OK {
				successes.Add(1)
				work[index].Add(1)
			}
		}(index)
	}
	ready.Wait()
	close(start)
	done.Wait()
	if successes.Load() != 1 || work[0].Load()+work[1].Load() != 1 {
		t.Fatalf("successes=%d work=(%d,%d) errors=(%v,%v)", successes.Load(), work[0].Load(), work[1].Load(), errs[0], errs[1])
	}
	winner := 0
	if results[1].OK {
		winner = 1
	}
	loser := 1 - winner
	if work[loser].Load() != 0 {
		t.Fatalf("losing receiver performed external work: winner=%d work=(%d,%d)", winner, work[0].Load(), work[1].Load())
	}
	stored, err := issueops.ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Execution.Lease.Holder == nil || stored.Execution.Lease.Holder.SessionID != actors[winner].SessionID {
		t.Fatalf("stored authority holder=%+v winner=%+v", stored.Execution.Lease.Holder, actors[winner])
	}
}

// seedDirectClaimFixture는 generation 1이 claimable인 direct 실행 레코드와 발급된
// claim 토큰 파일을 만든다.
func seedDirectClaimFixture(t *testing.T) (string, issueopscontract.IssueOpsRecord, string) {
	t.Helper()
	stateRoot := t.TempDir()
	source := filepath.Join(t.TempDir(), "source")
	worktree := filepath.Join(t.TempDir(), "worktree")
	const branch = "192-direct-claim"
	claimWiringGit(t, "", "init", "-q", "-b", "main", source)
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("# direct claim fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	claimWiringGit(t, source, "add", "README.md")
	claimWiringGit(t, source, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "-q", "-m", "test: direct claim fixture")
	claimWiringGit(t, source, "worktree", "add", "-q", "-b", branch, worktree, "main")
	baseHead := strings.TrimSpace(claimWiringGit(t, worktree, "rev-parse", "HEAD"))
	record, err := startIssueOpsFixture(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: source, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = issueopscontract.IssueOpsPhaseImplement
	record.IssueURL = "https://github.com/acme/repo/issues/16"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch, BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true}
	record.WorktreePath = worktree
	record.Execution = &issueopscontract.Execution{
		Workspace: issueopscontract.Workspace{SourceRoot: source, Root: worktree, Branch: record.Branch, BaseHead: baseHead, LinkedAt: "2026-07-30T09:00:00Z", ArtifactDir: ".issueops/issues/16/artifact"},
		Lease:     issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusClaimable},
	}
	const token = "direct-claim-token"
	record.Execution.Lease.ClaimTokenSHA256 = claimWiringSHA256(token)
	tokenPath := filepath.Join(worktree, ".issueops", "state", "issueops-v1", claimWiringSHA256(record.ID)[:16], "lease-1.token")
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenPath, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (issueops.CycleRecordStore{StateRoot: stateRoot}).Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	return stateRoot, record, tokenPath
}

func claimWiringActor(t *testing.T) issueopscontract.NativeActor {
	t.Helper()
	receipt, err := issueops.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	return issueopscontract.NativeActor{Host: "codex", SessionID: "claim-wiring", SessionProcess: &receipt, ProcessAncestry: []issueopscontract.NativeProcessReceipt{receipt}}
}

func claimWiringSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func claimWiringGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}
