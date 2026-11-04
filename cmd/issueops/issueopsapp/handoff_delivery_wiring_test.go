package issueopsapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	issueopsadapter "issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/testsupport"
)

func TestManualHandoffDeliveryRejectsClaimForgeryAndUsesIsolatedLineage(t *testing.T) {
	stateRoot := t.TempDir()
	eventNow := handoffDeliveryEventClock(time.Now)
	observation := manualHerdrHandoffObservation("io-manual-forgery", 1, "attempt-1", eventNow())
	record := issueopscontract.Record{
		ID:        observation.LifecycleID,
		Execution: &issueopscontract.Execution{Lease: issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusReleased}},
	}
	if err := issueopsdomain.ValidateManualHandoffDeliveryObservation(record, observation); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*issueopscontract.HandoffDeliveryObservation){
		func(value *issueopscontract.HandoffDeliveryObservation) {
			value.OwnerActor = &issueopscontract.NativeActor{Host: "codex", SessionID: "forged"}
		},
		func(value *issueopscontract.HandoffDeliveryObservation) {
			value.OwnerClaimed = handoffDeliveryObserved(eventNow, "issueops_claim")
		},
		func(value *issueopscontract.HandoffDeliveryObservation) {
			value.OwnerClaim = issueopscontract.HandoffDeliveryOwnerClaim{Claimed: true, Generation: 1, ClaimedAt: observation.UpdatedAt}
		},
	} {
		forged := observation
		mutate(&forged)
		if err := issueopsdomain.ValidateManualHandoffDeliveryObservation(record, forged); err == nil {
			t.Fatal("public manual observation accepted forged owner claim fields")
		}
	}
	if _, err := newHandoffDeliveryAudit(stateRoot).Append(observation); err != nil {
		t.Fatal(err)
	}
	standardLineage := strings.TrimPrefix(observation.LineageID, "manual-direct:")
	folded, _, err := foldHandoffAuditForTest(stateRoot, observation.LifecycleID, standardLineage)
	if err != nil || len(folded) != 0 {
		t.Fatalf("manual observation entered execution recovery lineage: folded=%+v err=%v", folded, err)
	}
}

func TestManualCmuxHandoffDeliveryAllowsOnlyReleasedDirectStaging(t *testing.T) {
	record := issueopscontract.Record{
		ID: "io-cmux-manual",
		Execution: &issueopscontract.Execution{
			Lease: issueopscontract.WriteLease{Generation: 3, Status: issueopscontract.LeaseStatusReleased},
		},
	}
	observation := manualCmuxHandoffObservation(record.ID, 3)
	if err := issueopsdomain.ValidateManualHandoffDeliveryObservation(record, observation); err != nil {
		t.Fatalf("valid cmux staging rejected: %v", err)
	}

	wrongGeneration := observation
	wrongGeneration.SourceGeneration++
	if err := issueopsdomain.ValidateManualHandoffDeliveryObservation(record, wrongGeneration); err == nil || !strings.Contains(err.Error(), "exact released direct execution generation") {
		t.Fatalf("wrong generation accepted: %v", err)
	}
	active := record
	active.Execution = &issueopscontract.Execution{Lease: issueopscontract.WriteLease{Generation: 3, Status: issueopscontract.LeaseStatusActive}}
	if err := issueopsdomain.ValidateManualHandoffDeliveryObservation(active, observation); err == nil || !strings.Contains(err.Error(), "exact released direct execution generation") {
		t.Fatalf("active generation accepted: %v", err)
	}
}

func TestPublicManualHandoffProducerRejectsEveryOwnerClaimField(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	stateRoot := issueOpsStateRoot()
	record := seedReleasedDirectHandoffRecord(t, stateRoot)
	eventNow := handoffDeliveryEventClock(time.Now)
	observation := manualHerdrHandoffObservation(record.ID, 1, "attempt-1", eventNow())
	for name, mutate := range map[string]func(*issueopscontract.HandoffDeliveryObservation){
		"owner_actor": func(value *issueopscontract.HandoffDeliveryObservation) {
			value.OwnerActor = &issueopscontract.NativeActor{Host: "codex", SessionID: "forged"}
		},
		"owner_claimed": func(value *issueopscontract.HandoffDeliveryObservation) {
			value.OwnerClaimed = handoffDeliveryObserved(eventNow, issueopscontract.HandoffDeliveryEvidenceIssueOpsClaim)
		},
		"owner_claim": func(value *issueopscontract.HandoffDeliveryObservation) {
			value.OwnerClaim = issueopscontract.HandoffDeliveryOwnerClaim{Claimed: true, Generation: 1, ClaimedAt: observation.UpdatedAt}
		},
	} {
		t.Run(name, func(t *testing.T) {
			forged := observation
			mutate(&forged)
			if _, err := newHandoffDeliveryService(issueOpsStateRoot()).ObserveManual(forged); err == nil || !strings.Contains(err.Error(), "cannot produce owner claim evidence") {
				t.Fatalf("public producer accepted forged %s: %v", name, err)
			}
		})
	}
	if _, err := newHandoffDeliveryService(issueOpsStateRoot()).ObserveManual(observation); err != nil {
		t.Fatalf("valid manual observation: %v", err)
	}
}

func TestPublicManualHandoffProducerUsesIssueOpsStateNamespace(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	stateRoot := issueOpsStateRoot()
	record := seedReleasedDirectHandoffRecord(t, stateRoot)
	observation := manualHerdrHandoffObservation(record.ID, 1, "namespace", time.Now())

	if _, err := newHandoffDeliveryService(issueOpsStateRoot()).ObserveManual(observation); err != nil {
		t.Fatalf("audit manual observation through production namespace: %v", err)
	}
	observations, err := newHandoffDeliveryAudit(stateRoot).Read()
	if err != nil || len(observations) != 1 {
		t.Fatalf("issueops namespace observations=%d err=%v", len(observations), err)
	}
	parentObservations, err := newHandoffDeliveryAudit(stateDir).Read()
	if err != nil || len(parentObservations) != 0 {
		t.Fatalf("parent namespace observations=%d err=%v", len(parentObservations), err)
	}
}

func manualCmuxHandoffObservation(lifecycleID string, generation uint64) issueopscontract.HandoffDeliveryObservation {
	created := "2026-09-20T10:00:01Z"
	return issueopscontract.HandoffDeliveryObservation{
		SchemaVersion:  issueopscontract.HandoffDeliverySchemaVersion,
		AttemptID:      "manual-direct:" + lifecycleID + ":3:cmux:attempt",
		LineageID:      "manual-direct:" + "generation:3:cmux:window:window-1",
		LifecycleID:    lifecycleID,
		PromptSHA256:   strings.Repeat("a", 64),
		MaterialSHA256: strings.Repeat("b", 64),
		Launcher: issueopscontract.HandoffDeliveryLauncher{
			Name: issueopscontract.HandoffDeliveryLauncherCmux, Version: "0.64.10",
			Path: cmuxPath,
			EndpointIncarnation: &issueopscontract.HandoffDeliveryEndpointIncarnation{
				Path: socketPath, Kind: "unix_socket", Device: 1, Inode: 2, CTimeNS: 3,
				OwnerUID: 501, OwnerGID: 20, Mode: 0o600, ParentPath: filepath.Dir(socketPath), ParentDevice: 1,
				ParentInode: 1, ParentOwnerUID: 0, ParentOwnerGID: 0, ParentMode: 0o1777,
			},
		},
		Target:             issueopscontract.HandoffDeliveryTarget{WindowID: "window-1", CWD: "/repo/worktree"},
		ExpectedOwnerHost:  "codex",
		SourceGeneration:   generation,
		CreatedAt:          created,
		UpdatedAt:          created,
		Receipt:            issueopscontract.HandoffDeliveryReceipt{Location: "audit/cmux.json", Digest: strings.Repeat("d", 64)},
		CallStaged:         issueopscontract.HandoffDeliveryState{Status: issueopscontract.HandoffDeliveryStateObserved, ObservedAt: created, Evidence: issueopscontract.HandoffDeliveryEvidenceExternalCallStaged},
		InputAccepted:      issueopscontract.HandoffDeliveryState{Status: issueopscontract.HandoffDeliveryStateNotObserved},
		NativeTurnObserved: issueopscontract.HandoffDeliveryState{Status: issueopscontract.HandoffDeliveryStateNotObserved},
		OwnerClaimed:       issueopscontract.HandoffDeliveryState{Status: issueopscontract.HandoffDeliveryStateNotObserved},
		Ambiguous:          issueopscontract.HandoffDeliveryState{Status: issueopscontract.HandoffDeliveryStateNotObserved},
	}
}

func seedReleasedDirectHandoffRecord(t *testing.T, stateRoot string) issueopscontract.Record {
	t.Helper()
	testsupport.IsolateGitConfig(t)
	repo := t.TempDir()
	claimWiringGit(t, repo, "init", "-q", "-b", "main")
	claimWiringGit(t, repo, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "--allow-empty", "-q", "-m", "initial")
	baseHead := strings.TrimSpace(claimWiringGit(t, repo, "rev-parse", "HEAD"))
	record, err := startIssueOpsFixture(stateRoot, issueopscontract.StartRequest{Repo: repo, Branch: "11-manual-handoff"})
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(t.TempDir(), "manual-handoff")
	if err := os.MkdirAll(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	record.Phase = issueopscontract.PhaseImplement
	record.WorktreePath = worktree
	record.IssueURL = "https://github.com/example/issueops/issues/11"
	record.BranchPrepare = &issueopscontract.BranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch,
		BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	record.Execution = &issueopscontract.Execution{
		Workspace: issueopscontract.Workspace{
			SourceRoot: repo, Root: worktree, Branch: record.Branch, BaseHead: baseHead,
			LinkedAt: time.Now().UTC().Format(time.RFC3339Nano),
		},
		Lease: issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusReleased},
	}
	written, err := (issueopsadapter.CycleRecordStore{StateRoot: stateRoot}).Save(context.Background(), record)
	if err != nil {
		t.Fatal(err)
	}
	return written
}
