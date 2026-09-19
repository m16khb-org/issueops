package issueopsapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	auditadapter "issueops/internal/adapter/audit"
	"issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestIssueOpsAppClaimWiring(t *testing.T) {
	_, err := issueOpsClaimHandler(context.Background(), t.TempDir(), issueops.ExecutionClaimRequest{ID: "io-claim-wiring"}, issueops.ExecutionClaimDependencies{})
	if err == nil || !strings.Contains(err.Error(), "issueops record io-claim-wiring not found") {
		t.Fatalf("claim wiring error=%v", err)
	}
}

func TestIssueOpsClaimProviderNameUsesBranchPrepareAuthority(t *testing.T) {
	got, err := issueOpsClaimProviderName(issueopscontract.IssueOpsRecord{
		IssueURL:      "https://code.company.example/group/issueops/-/issues/197",
		BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{Provider: "gitlab"},
	})
	if err != nil || got != "gitlab" {
		t.Fatalf("provider=%q err=%v", got, err)
	}
}

func TestIssueOpsClaimProviderNameRejectsURLInferenceWithoutBranchAuthority(t *testing.T) {
	_, err := issueOpsClaimProviderName(issueopscontract.IssueOpsRecord{})
	if err == nil || !strings.Contains(err.Error(), "linked issue provider is unavailable") {
		t.Fatalf("URL inference must be rejected: %v", err)
	}
}

func TestIssueOpsClaimHandlerUsesResolvedSnapshotReader(t *testing.T) {
	stateRoot, record, token, issueDigest, packetDigest := seedOrcaClaimSnapshot(t)
	reads := 0
	result, err := issueOpsClaimHandler(context.Background(), stateRoot, issueops.ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: claimWiringActor(t), CWD: record.Execution.Workspace.Root,
		TokenFile: token, IssueBodySHA256: issueDigest, ContextPacketSHA256: packetDigest,
	}, issueops.ExecutionClaimDependencies{ReadIssue: func(_ context.Context, providerName string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		reads++
		if providerName != "gitlab" || request.URL != record.IssueURL {
			t.Fatalf("snapshot request provider=%q url=%q", providerName, request.URL)
		}
		return port.ExecutionIssueSnapshot{URL: request.URL, Body: claimWiringIssueBody()}, nil
	},
	})
	if err != nil {
		t.Fatalf("claim with resolved snapshot reader: %v", err)
	}
	if !result.OK || result.Execution.Lease.Status != issueopscontract.LeaseStatusActive || reads != 1 {
		t.Fatalf("claim result=%+v resolved_reads=%d", result, reads)
	}
}

func TestIssueOpsClaimProducesOwnerClaimEvidenceFromCommittedLease(t *testing.T) {
	for _, host := range []string{"codex", "claude", "omo"} {
		t.Run(host, func(t *testing.T) {
			stateRoot, record, token, issueDigest, packetDigest := seedOrcaClaimSnapshot(t)
			record.Execution.Orca.OwnerHost = host
			if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			seedClaimDeliveryObservation(t, stateRoot, record)
			actor := claimWiringActor(t)
			actor.Host = host
			result, err := issueOpsClaimHandler(context.Background(), stateRoot, issueops.ExecutionClaimRequest{
				ID: record.ID, Generation: 1, Actor: actor, CWD: record.Execution.Workspace.Root,
				TokenFile: token, IssueBodySHA256: issueDigest, ContextPacketSHA256: packetDigest,
			}, claimWiringDependencies(record))
			if err != nil {
				t.Fatal(err)
			}
			callKind := "dispatch"
			if host == "omo" {
				callKind = "prompt"
			}
			lineageID := "generation:1:prompt:" + record.Execution.Orca.OwnerPromptSHA256 + ":material:" + record.Execution.Orca.ContextPacketSHA256 + ":call:" + callKind
			folded, _, err := auditadapter.FoldHandoffDeliveryAuditObservationsForAt(stateRoot, record.ID, lineageID)
			if err != nil {
				t.Fatal(err)
			}
			observation := folded[record.ID+"\x00"+lineageID]
			if observation.OwnerClaimed.Status != issueopscontract.IssueOpsHandoffDeliveryStateObserved || !observation.OwnerClaim.Claimed || observation.OwnerActor == nil || observation.OwnerActor.SessionID != actor.SessionID || observation.OwnerClaim.Generation != result.Execution.Lease.Generation {
				t.Fatalf("owner claim observation=%+v", observation)
			}
		})
	}
}

func TestSuccessfulDirectClaimAttachesOnlyToExactManualReceiverProcess(t *testing.T) {
	for _, test := range []struct {
		name      string
		mismatch  bool
		wantClaim bool
	}{
		{name: "exact process", wantClaim: true},
		{name: "process mismatch", mismatch: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			request := handoffDeliveryRequestFixture("codex")
			identity := handoffDeliveryIdentityFixture()
			at := time.Now().UTC().Add(-time.Minute)
			eventNow := handoffDeliveryEventClock(func() time.Time { return at })
			observation, err := handoffDeliveryObservation(stateRoot, request, identity, port.ExecutionOrcaIntentReceipt{}, "", "prompt", eventNow)
			if err != nil {
				t.Fatal(err)
			}
			observation.AttemptID = handoffDeliveryManualLineagePrefix + request.Workspace.LifecycleID + ":1:orca:claim"
			observation.LineageID = handoffDeliveryManualLineagePrefix + observation.LineageID
			actor := claimWiringActor(t)
			process := *actor.SessionProcess
			observation.Target.Process = &process
			observation.InputAccepted = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherReceipt)
			if _, err := auditadapter.AuditHandoffDeliveryObservationAt(stateRoot, observation); err != nil {
				t.Fatal(err)
			}
			if test.mismatch {
				actor.SessionProcess = &issueopscontract.NativeProcessReceipt{PID: process.PID + 1, StartedAt: process.StartedAt, Executable: process.Executable}
			}
			claimedAt := time.Now().UTC().Format(time.RFC3339Nano)
			result := issueops.ExecutionResult{OK: true, ID: request.Workspace.LifecycleID, Execution: issueopscontract.Execution{
				Mode:  issueopscontract.ExecutionModeDirect,
				Lease: issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusActive, Holder: &actor, ClaimedAt: claimedAt},
			}}
			if err := observeSuccessfulIssueOpsClaim(stateRoot, result); err != nil {
				t.Fatal(err)
			}
			folded, _, err := auditadapter.FoldHandoffDeliveryAuditObservationsForAt(stateRoot, observation.LifecycleID, observation.LineageID)
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

func TestSuccessfulDirectClaimRejectsAmbiguousManualLineages(t *testing.T) {
	stateRoot := t.TempDir()
	request := handoffDeliveryRequestFixture("codex")
	identity := handoffDeliveryIdentityFixture()
	at := time.Now().UTC().Add(-time.Minute)
	eventNow := handoffDeliveryEventClock(func() time.Time { return at })
	actor := claimWiringActor(t)
	for _, suffix := range []string{"first", "second"} {
		observation, err := handoffDeliveryObservation(stateRoot, request, identity, port.ExecutionOrcaIntentReceipt{}, "", "prompt", eventNow)
		if err != nil {
			t.Fatal(err)
		}
		observation.AttemptID = handoffDeliveryManualLineagePrefix + request.Workspace.LifecycleID + ":1:orca:" + suffix
		observation.LineageID = handoffDeliveryManualLineagePrefix + suffix + ":" + observation.LineageID
		process := *actor.SessionProcess
		observation.Target.Process = &process
		observation.InputAccepted = handoffDeliveryObserved(eventNow, issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherReceipt)
		if _, err := auditadapter.AuditHandoffDeliveryObservationAt(stateRoot, observation); err != nil {
			t.Fatal(err)
		}
	}
	result := issueops.ExecutionResult{OK: true, ID: request.Workspace.LifecycleID, Execution: issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Lease: issueopscontract.WriteLease{
			Generation: 1, Status: issueopscontract.LeaseStatusActive, Holder: &actor,
			ClaimedAt: time.Now().UTC().Format(time.RFC3339Nano),
		},
	}}
	if err := observeSuccessfulIssueOpsClaim(stateRoot, result); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous manual lineages were accepted: %v", err)
	}
}

func TestIssueOpsClaimDoesNotReturnFalseFailureWhenObservationAuditIsUnsafe(t *testing.T) {
	stateRoot, record, token, issueDigest, packetDigest := seedOrcaClaimSnapshot(t)
	seedClaimDeliveryObservation(t, stateRoot, record)
	path := filepath.Join(stateRoot, "audit", "handoff-delivery.jsonl")
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := issueOpsClaimHandler(context.Background(), stateRoot, issueops.ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: claimWiringActor(t), CWD: record.Execution.Workspace.Root,
		TokenFile: token, IssueBodySHA256: issueDigest, ContextPacketSHA256: packetDigest,
	}, claimWiringDependencies(record))
	if err != nil || !result.OK || result.Execution.Lease.Status != issueopscontract.LeaseStatusActive {
		t.Fatalf("committed claim was reported as failed: result=%+v err=%v", result, err)
	}
}

func TestIssueOpsConcurrentReceiversHaveOneAuthorityHolderAndLoserDoesNoWork(t *testing.T) {
	stateRoot, record, token, issueDigest, packetDigest := seedOrcaClaimSnapshot(t)
	actors := []issueopscontract.NativeActor{claimWiringActor(t), claimWiringActor(t)}
	actors[0].SessionID = "receiver-a"
	actors[1].SessionID = "receiver-b"
	var ready sync.WaitGroup
	ready.Add(2)
	start := make(chan struct{})
	var successes atomic.Int32
	var work [2]atomic.Int32
	errs := make([]error, 2)
	results := make([]issueops.ExecutionResult, 2)
	var done sync.WaitGroup
	done.Add(2)
	for index := range actors {
		go func(index int) {
			defer done.Done()
			ready.Done()
			<-start
			results[index], errs[index] = issueOpsClaimHandler(context.Background(), stateRoot, issueops.ExecutionClaimRequest{
				ID: record.ID, Generation: 1, Actor: actors[index], CWD: record.Execution.Workspace.Root,
				TokenFile: token, IssueBodySHA256: issueDigest, ContextPacketSHA256: packetDigest,
			}, claimWiringDependencies(record))
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

func claimWiringDependencies(record issueopscontract.IssueOpsRecord) issueops.ExecutionClaimDependencies {
	return issueops.ExecutionClaimDependencies{ReadIssue: func(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		return port.ExecutionIssueSnapshot{URL: request.URL, Body: claimWiringIssueBody()}, nil
	}}
}

func seedClaimDeliveryObservation(t *testing.T, stateRoot string, record issueopscontract.IssueOpsRecord) {
	t.Helper()
	created := time.Now().UTC().Add(-time.Minute)
	updated := created.Add(time.Second)
	callKind := "dispatch"
	if record.Execution.Orca.OwnerHost == "omo" {
		callKind = "prompt"
	}
	lineageID := "generation:1:prompt:" + record.Execution.Orca.OwnerPromptSHA256 + ":material:" + record.Execution.Orca.ContextPacketSHA256 + ":call:" + callKind
	observation := issueopscontract.IssueOpsHandoffDeliveryObservation{
		SchemaVersion: issueopscontract.IssueOpsHandoffDeliverySchemaVersion, AttemptID: "claim-delivery-attempt", LineageID: lineageID,
		LifecycleID: record.ID, PromptSHA256: record.Execution.Orca.OwnerPromptSHA256, MaterialSHA256: record.Execution.Orca.ContextPacketSHA256,
		Request:           issueopscontract.IssueOpsHandoffDeliveryRequest{DurableID: "22222222-2222-4222-8222-222222222222"},
		Launcher:          issueopscontract.IssueOpsHandoffDeliveryLauncher{Name: "orca", Version: "1.4.200", Path: "/usr/local/bin/orca", RuntimeID: record.Execution.Orca.RuntimeID, MachineID: "machine-claim", ServerID: "local"},
		Target:            issueopscontract.IssueOpsHandoffDeliveryTarget{TerminalID: "pty-claim", PaneID: "term-claim", ProcessIncarnation: "process-claim"},
		ExpectedOwnerHost: record.Execution.Orca.OwnerHost, SourceGeneration: 1, CreatedAt: created.Format(time.RFC3339Nano), UpdatedAt: updated.Format(time.RFC3339Nano),
		InputAccepted:      issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateObserved, ObservedAt: updated.Format(time.RFC3339Nano), Evidence: issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherReceipt},
		NativeTurnObserved: issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved},
		OwnerClaimed:       issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved},
		Ambiguous:          issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved},
	}
	if _, err := auditadapter.AuditHandoffDeliveryObservationAt(stateRoot, observation); err != nil {
		t.Fatal(err)
	}
}

func seedOrcaClaimSnapshot(t *testing.T) (string, issueopscontract.IssueOpsRecord, string, string, string) {
	t.Helper()
	stateRoot := t.TempDir()
	source := filepath.Join(t.TempDir(), "source")
	worktree := filepath.Join(t.TempDir(), "worktree")
	const branch = "192-snapshot-claim"
	claimWiringGit(t, "", "init", "-q", "-b", "main", source)
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("# snapshot fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	claimWiringGit(t, source, "add", "README.md")
	claimWiringGit(t, source, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "-q", "-m", "test: snapshot fixture")
	claimWiringGit(t, source, "worktree", "add", "-q", "-b", branch, worktree, "main")
	baseHead := strings.TrimSpace(claimWiringGit(t, worktree, "rev-parse", "HEAD"))
	record, err := issueops.StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: source, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = issueops.IssueOpsPhaseImplement
	record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/16"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{Provider: "gitlab", IssueURL: record.IssueURL, Branch: record.Branch, BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true}
	const plan = "# Snapshot owner plan\n"
	if _, err := stageIssueOpsArtifact(stateRoot, record.ID, "plan", []byte(plan)); err != nil {
		t.Fatal(err)
	}
	seedPlannerGates(t, stateRoot, record.ID)
	record.WorktreePath = worktree
	record.PlanPath = filepath.Join(worktree, "plans", "linked.md")
	if err := os.MkdirAll(filepath.Dir(record.PlanPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(record.PlanPath, []byte(plan), 0o600); err != nil {
		t.Fatal(err)
	}
	sealedPlanPath := filepath.Join(worktree, filepath.FromSlash(issueops.IssueOpsArtifactDir), "plan.md")
	if err := os.MkdirAll(filepath.Dir(sealedPlanPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sealedPlanPath, []byte(plan), 0o600); err != nil {
		t.Fatal(err)
	}
	record.Execution = &issueopscontract.Execution{
		Mode:      issueopscontract.ExecutionModeOrca,
		Workspace: issueopscontract.Workspace{SourceRoot: source, Root: worktree, Branch: record.Branch, BaseHead: baseHead, Driver: "orca", LinkedAt: "2026-07-30T09:00:00Z"},
		Lease:     issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusClaimable},
		Orca:      &issueopscontract.OrcaBinding{RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", LeaseGeneration: 1, OwnerHost: "codex", OwnerModel: "model", TaskID: "task", DispatchID: "dispatch"},
	}
	token := "snapshot-claim-token"
	tokenDigest := claimWiringSHA256(token)
	record.Execution.Lease.ClaimTokenSHA256 = tokenDigest
	key := claimWiringSHA256(record.ID)[:16]
	tokenPath := filepath.Join(worktree, ".issueops", "state", "issueops-v1", key, "lease-1.token")
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenPath, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	issueDigest := claimWiringSHA256(claimWiringIssueBody())
	packetPath := issueops.SealedOwnerContextPacketPath(record)
	packet := map[string]any{
		"schema_version": 1, "lifecycle_id": record.ID, "mode": "orca", "source_root": source, "worktree_root": worktree,
		"branch": record.Branch, "base_head": record.Execution.Workspace.BaseHead, "lease_generation": uint64(1), "claim_token_file": tokenPath,
		"issue": map[string]string{"url": record.IssueURL, "body": claimWiringIssueBody(), "body_sha256": issueDigest}, "artifact_manifest": map[string]string{"plan": claimWiringSHA256(plan)},
	}
	packetBytes, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(packetPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, packetBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	packetDigest := claimWiringSHA256(string(packetBytes))
	record.Execution.Orca.ArtifactIdentityVersion = issueopscontract.OrcaArtifactIdentityVersion
	record.Execution.Orca.IssueBodySHA256 = issueDigest
	record.Execution.Orca.ContextPacketSHA256 = packetDigest
	record.Execution.Orca.OwnerPromptSHA256 = strings.Repeat("d", 64)
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	return stateRoot, record, tokenPath, issueDigest, packetDigest
}

func claimWiringActor(t *testing.T) issueopscontract.NativeActor {
	t.Helper()
	receipt, err := issueops.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	return issueopscontract.NativeActor{Host: "codex", SessionID: "claim-wiring", SessionProcess: &receipt, ProcessAncestry: []issueopscontract.NativeProcessReceipt{receipt}}
}

func claimWiringIssueBody() string {
	return "## acceptance criteria\n\n- [ ] AC-09: resolved snapshot reader\n\n## verification\n\n```bash\ngo test ./cmd/issueops/issueopsapp -run Snapshot -count=1\n```\n"
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
