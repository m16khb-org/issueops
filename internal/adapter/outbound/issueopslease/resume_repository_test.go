package issueopslease

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	leasedomain "issueops/internal/domain/issueopslease"
	preparationdomain "issueops/internal/domain/issueopspreparation"
	"issueops/internal/port"
)

func TestResumeRepositoryLoadsExactGenerationSnapshot(t *testing.T) {
	_, store := newResumeRepositoryStore(t, resumeRepositoryRecord(t, 4))
	repository := NewResumeRepository(store)

	snapshot, err := repository.LoadSnapshot(context.Background(), "io-resume-repository", 4)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Record.Lease.Generation != 4 || len(snapshot.Raw) == 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestResumeRepositoryBeginIntentPersistsSealedPendingStateWithRawCAS(t *testing.T) {
	record := resumeRepositoryRecord(t, 4)
	repo := filepath.Join(t.TempDir(), "repo")
	record.Repo, record.Branch = repo, "193-resume"
	record.BranchPrepare = []byte(`{"provider":"github","issue_url":"https://github.com/m16khb/issueops/issues/193","link_verified":true,"base_branch":"main","base_sha":"base"}`)
	record.Execution.Workspace.SourceRoot = repo
	record.Execution.Workspace.Root = filepath.Join(repo+".worktrees", record.Branch)
	record.Execution.Workspace.Branch = record.Branch
	record.Execution.Workspace.BaseHead = "base"
	_, store := newResumeRepositoryStore(t, record)
	repository := NewResumeRepository(store)
	repository.now = func() time.Time { return time.Date(2026, time.July, 31, 3, 15, 0, 0, time.UTC) }
	snapshot, err := repository.LoadSnapshot(context.Background(), record.ID, 4)
	if err != nil {
		t.Fatal(err)
	}
	operationID := strings.Repeat("e", 32)
	artifacts := leasecontract.ResumeArtifacts{IssueBodySHA256: strings.Repeat("a", 64), OwnerPromptPath: filepath.Join(record.Execution.Workspace.Root, "prompt"), OwnerPromptSHA256: strings.Repeat("c", 64), ContextPacketPath: filepath.Join(record.Execution.Workspace.Root, "packet"), ContextPacketSHA256: strings.Repeat("d", 64)}
	progress, err := repository.BeginIntent(context.Background(), snapshot, artifacts, resumeRepositoryPlan(), operationID)
	if err != nil {
		t.Fatal(err)
	}
	if !progress.Pending || progress.Execution.Pending == nil || progress.Execution.Pending.OperationID != operationID || progress.Execution.Pending.Kind != "owner_launch" {
		t.Fatalf("progress=%+v", progress)
	}
	if _, ok, err := store.Get("external_intent_v1", operationID); err != nil || !ok {
		t.Fatalf("intent persisted=%v err=%v", ok, err)
	}
	if _, err := repository.BeginIntent(context.Background(), snapshot, artifacts, resumeRepositoryPlan(), strings.Repeat("f", 32)); err == nil {
		t.Fatal("stale raw snapshot was accepted")
	}
}

func TestResumeRepositoryMarkInvokingUsesRawCAS(t *testing.T) {
	repository, state, _ := seededResumeIntent(t)
	next, err := repository.MarkInvoking(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if next.InvocationState != preparationcontract.InvocationUnknown || next.InvocationAttempts != 1 {
		t.Fatalf("next invocation=%q attempts=%d", next.InvocationState, next.InvocationAttempts)
	}
	if _, err := repository.MarkInvoking(context.Background(), state); err == nil {
		t.Fatal("stale raw intent was accepted")
	}
}

func TestResumeRepositoryApplyReceiptAdvancesIntentWithRawCAS(t *testing.T) {
	repository, state, store := seededResumeIntent(t)
	progress, err := repository.ApplyReceipt(context.Background(), state, leasecontract.ResumeStageReceipt{TerminalPTYID: "pty-new"})
	if err != nil {
		t.Fatal(err)
	}
	if !progress.Pending || progress.Execution.Pending == nil || progress.Execution.Pending.Kind != preparationdomain.PendingKind(preparationcontract.IntentStageRun) {
		t.Fatalf("progress=%+v", progress)
	}
	loaded, err := repository.LoadIntent(context.Background(), progress)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Stage != string(preparationcontract.IntentStageRun) {
		t.Fatalf("stage=%s", loaded.Stage)
	}
	if _, err := repository.ApplyReceipt(context.Background(), state, leasecontract.ResumeStageReceipt{TerminalPTYID: "pty-again"}); err == nil {
		t.Fatal("stale receipt snapshot was accepted")
	}
	data, ok, err := store.Get("external_intent_v1", state.OperationID)
	if err != nil || !ok || string(data) != string(loaded.IntentRaw) {
		t.Fatalf("persisted intent: present=%v err=%v", ok, err)
	}
}

func TestResumeRepositoryApplyDispatchReceiptPreservesLeaseAndDeletesIntent(t *testing.T) {
	repository, state, store := seededResumeIntent(t)
	payload, err := (preparationcontract.IntentCodec{}).Decode(state.OperationID, state.IntentRaw)
	if err != nil {
		t.Fatal(err)
	}
	payload.Stage = preparationcontract.IntentStageDispatch
	payload.TerminalPTYID, payload.RunID, payload.RunBound, payload.TaskID = "pty-new", "run-new", true, "task-new"
	intentRaw, err := (preparationcontract.IntentCodec{}).Encode(payload)
	if err != nil {
		t.Fatal(err)
	}
	record := state.Progress.Record.Stable
	record.Execution.Pending.Kind = preparationdomain.PendingKind(payload.Stage)
	recordRaw, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(context.Background(), []port.RecordMutation{
		{Bucket: recordBucket, ID: record.ID, Data: recordRaw},
		{Bucket: "external_intent_v1", ID: state.OperationID, Data: intentRaw},
	}); err != nil {
		t.Fatal(err)
	}
	state.Progress.Record = toApplicationRecord(record)
	state.Progress.Execution = *record.Execution
	state.Stage = string(payload.Stage)
	state.RecordRaw, state.IntentRaw = recordRaw, intentRaw
	lease := record.Execution.Lease
	if _, err := repository.ApplyReceipt(context.Background(), state, leasecontract.ResumeStageReceipt{TaskID: "task-new", DispatchID: "dispatch-new"}); err == nil {
		t.Fatal("incomplete dispatch receipt was accepted")
	}
	if data, ok, err := store.Get(recordBucket, record.ID); err != nil || !ok || string(data) != string(recordRaw) {
		t.Fatalf("invalid dispatch changed record: present=%v err=%v", ok, err)
	}
	if data, ok, err := store.Get("external_intent_v1", state.OperationID); err != nil || !ok || string(data) != string(intentRaw) {
		t.Fatalf("invalid dispatch changed intent: present=%v err=%v", ok, err)
	}
	progress, err := repository.ApplyReceipt(context.Background(), state, leasecontract.ResumeStageReceipt{
		TerminalPTYID: "pty-new", TerminalHandle: "handle-new", TaskID: "task-new",
		DispatchID: "dispatch-new", RequestID: "11111111-1111-4111-8111-111111111111",
	})
	if err != nil {
		t.Fatal(err)
	}
	if progress.Pending || progress.Execution.Pending != nil || progress.Execution.Orca == nil || progress.Execution.Orca.DispatchID != "dispatch-new" || progress.Execution.Orca.RuntimeID != payload.Prepared.RuntimeID || progress.Execution.Lease != lease {
		t.Fatalf("dispatch progress=%+v", progress)
	}
	if _, ok, err := store.Get("external_intent_v1", state.OperationID); err != nil || ok {
		t.Fatalf("completed intent remains: present=%v err=%v", ok, err)
	}
}

func TestResumeReceiptConversionPreservesPromptIdentity(t *testing.T) {
	sequence := uint64(0)
	receipt := leasecontract.ResumeStageReceipt{
		TerminalPTYID: "pty", RunID: "run", RunBound: true, TaskID: "task", DispatchID: "dispatch",
		RequestID: "11111111-1111-4111-8111-111111111111",
		PromptReceipt: &leasecontract.OrcaPromptReceipt{
			RequestID: "22222222-2222-4222-8222-222222222222", Provider: "omo",
			Stages: []string{"input_accepted"}, ProcessIncarnation: "process", Generation: 4,
			BaselineWorkingSequence: &sequence,
		},
	}
	portReceipt := resumePortReceipt(receipt)
	preparationReceipt := resumePreparationReceipt(receipt)
	if portReceipt.RunID != receipt.RunID || !portReceipt.RunBound || portReceipt.PromptReceipt == nil || portReceipt.PromptReceipt.BaselineWorkingSequence == nil || *portReceipt.PromptReceipt.BaselineWorkingSequence != 0 {
		t.Fatalf("port receipt=%+v", portReceipt)
	}
	if preparationReceipt.PromptReceipt == nil || preparationReceipt.PromptReceipt.RequestID != receipt.PromptReceipt.RequestID || preparationReceipt.PromptReceipt.BaselineWorkingSequence == nil {
		t.Fatalf("preparation receipt=%+v", preparationReceipt)
	}
}

func TestResumeRepositoryLoadsPendingIntentFromStore(t *testing.T) {
	repository, state, store := seededResumeIntent(t)
	loaded, err := repository.LoadIntent(context.Background(), state.Progress)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.OperationID != state.OperationID || loaded.Stage != state.Stage || string(loaded.RecordRaw) != string(state.RecordRaw) || string(loaded.IntentRaw) != string(state.IntentRaw) {
		t.Fatalf("loaded=%+v", loaded)
	}
	if err := store.Apply(context.Background(), []port.RecordMutation{{Bucket: "external_intent_v1", ID: state.OperationID, Delete: true}}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.LoadIntent(context.Background(), state.Progress); err == nil || !strings.Contains(err.Error(), "Orca external intent payload is missing") {
		t.Fatalf("missing intent error=%v", err)
	}
}

func TestResumeRepositoryRecordFailureUsesRawCASAndAdoptsRequestIDs(t *testing.T) {
	repository, state, store := seededResumeIntent(t)
	repository.now = func() time.Time { return time.Date(2026, time.July, 31, 3, 0, 0, 0, time.UTC) }
	repository.redact = func(string) string { return "redacted failure" }
	cause := &port.OrcaError{CallPhase: "terminal_send", DispatchRequestID: "11111111-1111-4111-8111-111111111111", OrchestrationRequestID: "22222222-2222-4222-8222-222222222222"}
	if err := repository.RecordFailure(context.Background(), state, preparationcontract.InvocationUnknown, cause); err != nil {
		t.Fatal(err)
	}
	data, ok, err := store.Get(recordBucket, state.Progress.Record.ID)
	if err != nil || !ok {
		t.Fatalf("record: present=%v err=%v", ok, err)
	}
	record, err := decodeMutableLeaseRecord(state.Progress.Record.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	if got := record.Execution.Failure; got == nil || got.Message != "redacted failure" || got.OperationID != state.OperationID || got.At != "2026-07-31T03:00:00Z" {
		t.Fatalf("failure=%+v", got)
	}
	data, ok, err = store.Get("external_intent_v1", state.OperationID)
	if err != nil || !ok {
		t.Fatalf("intent: present=%v err=%v", ok, err)
	}
	intent, err := (preparationcontract.IntentCodec{}).Decode(state.OperationID, data)
	if err != nil {
		t.Fatal(err)
	}
	if intent.OrcaRequestID != cause.DispatchRequestID || intent.OrcaPromptRequestID != cause.OrchestrationRequestID || intent.InvocationState != preparationcontract.InvocationUnknown {
		t.Fatalf("intent=%+v", intent)
	}
	if err := repository.RecordFailure(context.Background(), state, preparationcontract.InvocationUnknown, cause); err == nil {
		t.Fatal("stale raw snapshot was accepted")
	}
}

func seededResumeIntent(t *testing.T) (*ResumeRepository, leaseapp.ResumeIntentState, *sqlstore.DB) {
	t.Helper()
	repository, state, store, _ := seededResumeIntentAt(t)
	return repository, state, store
}

func seededResumeIntentAt(t *testing.T) (*ResumeRepository, leaseapp.ResumeIntentState, *sqlstore.DB, string) {
	t.Helper()
	const operationID = "0123456789abcdef0123456789abcdef"
	record := resumeRepositoryRecord(t, 4)
	lease := record.Execution.Lease
	binding := record.Execution.Orca
	intent := preparationcontract.Intent{
		SchemaVersion: leasecontract.SchemaVersion, Purpose: preparationcontract.PurposeResume,
		OperationID: operationID, LifecycleID: record.ID, Generation: lease.Generation,
		Stage: preparationcontract.IntentStageTerminal, StartedAt: "2026-07-31T00:00:00Z",
		InvocationState: preparationcontract.InvocationNotInvoked,
		Workspace: preparationcontract.WorkspaceRequest{
			LifecycleID: record.ID, SourceRoot: record.Execution.Workspace.SourceRoot,
			Root: record.Execution.Workspace.Root, Branch: record.Execution.Workspace.Branch,
			BaseHead: record.Execution.Workspace.BaseHead,
		},
		Probe: preparationcontract.ProbeRequest{Repo: record.Execution.Workspace.SourceRoot, Host: binding.OwnerHost, Model: binding.OwnerModel},
		Prepared: &preparationcontract.OrcaWorkspaceReceipt{
			Workspace: preparationcontract.WorkspaceReceipt{
				SourceRoot: record.Execution.Workspace.SourceRoot, Root: record.Execution.Workspace.Root,
				Branch: record.Execution.Workspace.Branch, BaseHead: record.Execution.Workspace.BaseHead, Driver: "orca", Exists: true,
			},
			RuntimeID: binding.RuntimeID, RepoID: binding.RepoID, WorktreeID: binding.WorktreeID,
		},
		Launch: &preparationcontract.LaunchIdentity{
			PromptPath: "/worktree/prompt", PromptSHA256: strings.Repeat("c", 64),
			ContextPacketPath: "/worktree/packet", ContextPacketSHA256: strings.Repeat("d", 64),
		},
		IssueBodySHA256: strings.Repeat("e", 64), ClaimTokenSHA256: lease.ClaimTokenSHA256,
		ResumeLease: &lease,
		PriorBinding: &preparationcontract.ResumeBinding{
			RuntimeID: binding.RuntimeID, RepoID: binding.RepoID, WorktreeID: binding.WorktreeID,
			LeaseGeneration: binding.LeaseGeneration, OwnerHost: binding.OwnerHost,
			OwnerModel: binding.OwnerModel, OwnerEffort: binding.OwnerEffort,
			RunID: binding.RunID, TaskID: binding.TaskID, DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID,
		},
	}
	intent, err := preparationdomain.SealIntent(intent, preparationcontract.IssueIdentity{Provider: "github", Issue: 193})
	if err != nil {
		t.Fatal(err)
	}
	record.Execution.Pending = &leasecontract.ExternalIntent{OperationID: operationID, Kind: "owner_launch", Marker: intent.Marker, StartedAt: intent.StartedAt}
	stateRoot, store := newResumeRepositoryStore(t, record)
	intentRaw, err := (preparationcontract.IntentCodec{}).Encode(intent)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(context.Background(), []port.RecordMutation{{Bucket: "external_intent_v1", ID: operationID, Data: intentRaw}}); err != nil {
		t.Fatal(err)
	}
	recordRaw, ok, err := store.Get(recordBucket, record.ID)
	if err != nil || !ok {
		t.Fatalf("record raw: present=%v err=%v", ok, err)
	}
	repository := NewResumeRepository(store)
	state := leaseapp.ResumeIntentState{
		Progress:    leaseapp.ResumeProgress{Record: toApplicationRecord(record), Execution: *record.Execution, Pending: true},
		OperationID: operationID, Stage: string(intent.Stage), InvocationState: intent.InvocationState,
		RecordRaw: recordRaw, IntentRaw: intentRaw,
	}
	return repository, state, store, stateRoot
}

func newResumeRepositoryStore(t *testing.T, record leasecontract.Record) (string, *sqlstore.DB) {
	t.Helper()
	stateRoot := t.TempDir()
	store, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(context.Background(), []port.RecordMutation{resumeRepositoryMutation(t, record)}); err != nil {
		t.Fatal(err)
	}
	return stateRoot, store
}

func resumeRepositoryMutation(t *testing.T, record leasecontract.Record) port.RecordMutation {
	t.Helper()
	data, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	return port.RecordMutation{Bucket: recordBucket, ID: record.ID, Data: data}
}

func resumeRepositoryRecord(t *testing.T, generation uint64) leasecontract.Record {
	t.Helper()
	return leasecontract.Record{OK: true, SchemaVersion: leasecontract.SchemaVersion, ID: "io-resume-repository", Repo: "m16khb/issueops", IssueURL: "https://github.com/m16khb/issueops/issues/193", BranchPrepare: []byte(`{"provider":"github","issue_url":"https://github.com/m16khb/issueops/issues/193","link_verified":true}`), Phase: "implement", CreatedAt: "2026-07-31T00:00:00Z", UpdatedAt: "2026-07-31T00:00:00Z", Execution: &leasecontract.Execution{
		Mode: "orca", Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "193-resume", BaseHead: "c30fb6761a24eae102f9e79e043306e60525207d", Driver: "orca", LinkedAt: "2026-07-31T00:00:00Z"},
		Lease:     leasecontract.Lease{Generation: generation, Status: "claimable", ClaimTokenSHA256: strings.Repeat("b", 64)},
		Orca:      &leasecontract.OrcaBinding{RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", OwnerHost: "codex", OwnerModel: "gpt-6-astra", OwnerEffort: "xhigh", TaskID: "task", DispatchID: "dispatch", TerminalPTYID: "pty", LeaseGeneration: generation, RunID: "run_issueops_1", ArtifactIdentityVersion: 1, IssueBodySHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ContextPacketSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", OwnerPromptSHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
		Selection: leaseSelectionFixture("orca"),
	}}
}

func resumeRepositoryPlan() leasedomain.ResumePlan {
	return leasedomain.ResumePlan{Disposition: leasedomain.ResumeCreateTerminal, RuntimeID: "runtime"}
}
