package issueopslease

import (
	"context"
	"fmt"
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
	repository := NewResumeRepository(store, resumeEffectsFake{})

	snapshot, err := repository.LoadSnapshot(context.Background(), "io-resume-repository", 4)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Record.Lease.Generation != 4 || len(snapshot.Raw) == 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestResumeRepositoryPropagatesBridgeBeginFailure(t *testing.T) {
	_, store := newResumeRepositoryStore(t, resumeRepositoryRecord(t, 4))
	repository := NewResumeRepository(store, resumeEffectsFake{beginErr: fmt.Errorf("stale raw record snapshot")})
	snapshot, err := repository.LoadSnapshot(context.Background(), "io-resume-repository", 4)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.BeginIntent(context.Background(), snapshot, leasecontract.ResumeArtifacts{}, resumeRepositoryPlan(), strings.Repeat("a", 32))
	if err == nil || !strings.Contains(err.Error(), "stale raw record snapshot") {
		t.Fatalf("begin error=%v", err)
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
	record, err := decodeLeaseRecord(state.Progress.Record.ID, data)
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
			TaskID: binding.TaskID, DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID,
		},
	}
	intent, err := preparationdomain.SealIntent(intent, preparationcontract.IssueIdentity{Provider: "github", Issue: 193})
	if err != nil {
		t.Fatal(err)
	}
	record.Execution.Pending = &leasecontract.ExternalIntent{OperationID: operationID, Kind: "owner_launch", Marker: intent.Marker, StartedAt: intent.StartedAt}
	_, store := newResumeRepositoryStore(t, record)
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
	repository := NewResumeRepository(store, nil)
	state := leaseapp.ResumeIntentState{
		Progress:    leaseapp.ResumeProgress{Record: toApplicationRecord(record), Execution: *record.Execution, Pending: true},
		OperationID: operationID, Stage: string(intent.Stage), InvocationState: intent.InvocationState,
		RecordRaw: recordRaw, IntentRaw: intentRaw,
	}
	return repository, state, store
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
	return leasecontract.Record{OK: true, SchemaVersion: leasecontract.SchemaVersion, ID: "io-resume-repository", Repo: "m16khb/issueops", IssueURL: "https://github.com/m16khb/issueops/issues/193", Phase: "implement", CreatedAt: "2026-07-31T00:00:00Z", UpdatedAt: "2026-07-31T00:00:00Z", Execution: &leasecontract.Execution{
		Mode: "orca", Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "193-resume", BaseHead: "c30fb6761a24eae102f9e79e043306e60525207d", Driver: "orca", LinkedAt: "2026-07-31T00:00:00Z"},
		Lease: leasecontract.Lease{Generation: generation, Status: "claimable", ClaimTokenSHA256: strings.Repeat("b", 64)},
		Orca:  &leasecontract.OrcaBinding{RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", OwnerHost: "codex", OwnerModel: "gpt-5.6-terra", OwnerEffort: "xhigh", TaskID: "task", DispatchID: "dispatch", TerminalPTYID: "pty", LeaseGeneration: generation},
	}}
}

func resumeRepositoryPlan() leasedomain.ResumePlan {
	return leasedomain.ResumePlan{Disposition: leasedomain.ResumeCreateTerminal, RuntimeID: "runtime"}
}

type resumeEffectsFake struct{ beginErr error }

func (f resumeEffectsFake) Begin(context.Context, leasecontract.Record, []byte, leasecontract.ResumeArtifacts, leasedomain.ResumePlan, string) (ResumeEffectState, error) {
	return ResumeEffectState{}, f.beginErr
}
func (resumeEffectsFake) ApplyReceipt(context.Context, ResumeEffectState, leasecontract.ResumeStageReceipt) (ResumeEffectState, error) {
	return ResumeEffectState{}, nil
}
