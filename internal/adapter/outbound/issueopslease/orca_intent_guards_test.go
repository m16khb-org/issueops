package issueopslease

import (
	"context"
	"errors"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	"issueops/internal/port"
)

func TestBoundedOrcaFailureDiagnostic(t *testing.T) {
	cause := errors.New("  secret detail  ")
	tests := map[string]struct {
		cause  error
		redact func(string) string
		want   string
	}{
		"no redactor never leaks the raw cause": {cause: cause, want: "external operation failed"},
		"nil cause":                             {redact: func(string) string { return "unused" }, want: "external operation failed"},
		"blank redaction falls back":            {cause: cause, redact: func(string) string { return " \n" }, want: "external operation failed"},
		"redaction is trimmed":                  {cause: cause, redact: func(s string) string { return " <" + strings.TrimSpace(s) + "> " }, want: "<secret detail>"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := boundedOrcaFailureDiagnostic(test.cause, test.redact); got != test.want {
				t.Fatalf("got %q want %q", got, test.want)
			}
		})
	}
	long := strings.Repeat("x", 5000)
	if got := boundedOrcaFailureDiagnostic(cause, func(string) string { return long }); len(got) != 4096 {
		t.Fatalf("diagnostic length=%d want 4096", len(got))
	}
	exact := strings.Repeat("y", 4096)
	if got := boundedOrcaFailureDiagnostic(cause, func(string) string { return exact }); got != exact {
		t.Fatal("4096-byte diagnostic must be kept intact")
	}
}

func reconcileIntentFrom(state leaseapp.ResumeIntentState) leaseapp.ReconcileIntentState {
	return leaseapp.ReconcileIntentState{
		Progress:    leaseapp.ReconcileProgress{Record: state.Progress.Record.Stable, Pending: true, NextStage: state.Stage},
		OperationID: state.OperationID, Stage: state.Stage, InvocationState: state.InvocationState,
		InvocationAttempts: state.InvocationAttempts, RecordRaw: state.RecordRaw, IntentRaw: state.IntentRaw,
	}
}

// staleRecord rewrites the stored record so a raw snapshot taken earlier no longer matches.
func staleRecord(t *testing.T, store *sqlstore.DB, state leaseapp.ResumeIntentState) {
	t.Helper()
	record := state.Progress.Record.Stable
	record.UpdatedAt = "2026-08-01T00:00:00Z"
	if err := store.Apply(context.Background(), []port.RecordMutation{resumeRepositoryMutation(t, record)}); err != nil {
		t.Fatal(err)
	}
}

// staleIntent rewrites only the stored intent payload.
func staleIntent(t *testing.T, store *sqlstore.DB, state leaseapp.ResumeIntentState) {
	t.Helper()
	intent, err := (preparationcontract.IntentCodec{}).Decode(state.OperationID, state.IntentRaw)
	if err != nil {
		t.Fatal(err)
	}
	intent.InvocationAttempts++
	data, err := (preparationcontract.IntentCodec{}).Encode(intent)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(context.Background(), []port.RecordMutation{{Bucket: "external_intent_v1", ID: state.OperationID, Data: data}}); err != nil {
		t.Fatal(err)
	}
}

func TestMarkOrcaIntentInvokingDistinguishesStaleRecordFromStaleIntent(t *testing.T) {
	_, state, store := seededResumeIntent(t)
	staleRecord(t, store, state)
	_, _, err := markOrcaIntentInvoking(t.Context(), store, state.Progress.Record.Stable, state.OperationID, state.RecordRaw, state.IntentRaw)
	if err == nil || err.Error() != "stale raw record snapshot" {
		t.Fatalf("stale record err=%v", err)
	}

	_, state, store = seededResumeIntent(t)
	staleIntent(t, store, state)
	_, _, err = markOrcaIntentInvoking(t.Context(), store, state.Progress.Record.Stable, state.OperationID, state.RecordRaw, state.IntentRaw)
	if err == nil || err.Error() != "stale raw intent snapshot" {
		t.Fatalf("stale intent err=%v", err)
	}
}

func TestMarkOrcaIntentInvokingRejectsMissingEvidenceAndUnsupportedStores(t *testing.T) {
	_, state, store := seededResumeIntent(t)
	record := state.Progress.Record.Stable
	if _, _, err := markOrcaIntentInvoking(t.Context(), nil, record, state.OperationID, state.RecordRaw, state.IntentRaw); err == nil || !strings.Contains(err.Error(), "store is required") {
		t.Fatalf("nil store err=%v", err)
	}
	if _, _, err := markOrcaIntentInvoking(t.Context(), &claimStore{}, record, state.OperationID, state.RecordRaw, state.IntentRaw); err == nil || !strings.Contains(err.Error(), "does not support raw CAS") {
		t.Fatalf("plain store err=%v", err)
	}
	if _, _, err := markOrcaIntentInvoking(t.Context(), store, record, state.OperationID, state.RecordRaw, nil); err == nil || !strings.Contains(err.Error(), "raw CAS evidence is required") {
		t.Fatalf("missing intent raw err=%v", err)
	}
	if _, _, err := markOrcaIntentInvoking(t.Context(), store, record, state.OperationID, state.RecordRaw, []byte("{not json")); err == nil {
		t.Fatal("undecodable intent must fail")
	}
	if _, _, err := markOrcaIntentInvoking(t.Context(), store, record, state.OperationID, nil, state.IntentRaw); err == nil {
		t.Fatal("missing record raw must fail")
	}
}

func TestRecordOrcaIntentFailureRejectsStaleAndMissingEvidence(t *testing.T) {
	cause := errors.New("boom")
	_, state, store := seededResumeIntent(t)
	failureState := orcaFailureState{Record: state.Progress.Record.Stable, RecordRaw: state.RecordRaw, IntentRaw: state.IntentRaw, OperationID: state.OperationID}

	missing := failureState
	missing.IntentRaw = nil
	if err := recordOrcaIntentFailure(t.Context(), store, missing, "unknown", cause, nil, nil); err == nil || !strings.Contains(err.Error(), "raw CAS evidence is required") {
		t.Fatalf("missing intent raw err=%v", err)
	}
	corrupt := failureState
	corrupt.IntentRaw = []byte("{not json")
	if err := recordOrcaIntentFailure(t.Context(), store, corrupt, "unknown", cause, nil, nil); err == nil {
		t.Fatal("undecodable intent must fail")
	}

	staleIntent(t, store, state)
	if err := recordOrcaIntentFailure(t.Context(), store, failureState, "unknown", cause, nil, nil); err == nil || err.Error() != "stale raw intent snapshot" {
		t.Fatalf("stale intent err=%v", err)
	}

	_, state, store = seededResumeIntent(t)
	failureState = orcaFailureState{Record: state.Progress.Record.Stable, RecordRaw: state.RecordRaw, IntentRaw: state.IntentRaw, OperationID: state.OperationID}
	staleRecord(t, store, state)
	if err := recordOrcaIntentFailure(t.Context(), store, failureState, "unknown", cause, nil, nil); err == nil || err.Error() != "stale raw record snapshot" {
		t.Fatalf("stale record err=%v", err)
	}
}

func TestRecordOrcaIntentFailureDefaultsDiagnosticWithoutRedactor(t *testing.T) {
	_, state, store := seededResumeIntent(t)
	failureState := orcaFailureState{Record: state.Progress.Record.Stable, RecordRaw: state.RecordRaw, IntentRaw: state.IntentRaw, OperationID: state.OperationID}
	if err := recordOrcaIntentFailure(t.Context(), store, failureState, "unknown", errors.New("token=abc123"), nil, nil); err != nil {
		t.Fatal(err)
	}
	data, ok, err := store.Get(recordBucket, state.Progress.Record.ID)
	if err != nil || !ok {
		t.Fatalf("record present=%v err=%v", ok, err)
	}
	record, err := decodeMutableLeaseRecord(state.Progress.Record.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	failure := record.Execution.Failure
	if failure == nil || failure.Message != "external operation failed" || strings.Contains(failure.Message, "abc123") || failure.At == "" {
		t.Fatalf("failure=%+v", failure)
	}
}

func TestResumeRepositoryGuardsMissingStoreAndRawCASSupport(t *testing.T) {
	_, state, _ := seededResumeIntent(t)
	ctx := context.Background()
	for name, repository := range map[string]*ResumeRepository{
		"nil repository": nil,
		"nil store":      NewResumeRepository(nil),
		"no raw CAS":     NewResumeRepository(&claimStore{}),
	} {
		t.Run(name, func(t *testing.T) {
			expectPersistence := func(op string, err error) {
				t.Helper()
				if failure, ok := errors.AsType[*leasecontract.Failure](err); !ok || failure.Code != leasecontract.FailurePersistence {
					t.Fatalf("%s err=%v", op, err)
				}
			}
			if name != "no raw CAS" {
				_, err := repository.LoadSnapshot(ctx, "io-x", 1)
				expectPersistence("LoadSnapshot", err)
				_, err = repository.LoadIntent(ctx, state.Progress)
				expectPersistence("LoadIntent", err)
			}
			_, err := repository.BeginIntent(ctx, leaseapp.ResumeSnapshot{Record: state.Progress.Record, Raw: state.RecordRaw}, leasecontract.ResumeArtifacts{}, resumeRepositoryPlan(), state.OperationID)
			expectPersistence("BeginIntent", err)
			_, err = repository.MarkInvoking(ctx, state)
			expectPersistence("MarkInvoking", err)
			expectPersistence("RecordFailure", repository.RecordFailure(ctx, state, "unknown", errors.New("x")))
			_, err = repository.ApplyReceipt(ctx, state, leasecontract.ResumeStageReceipt{})
			expectPersistence("ApplyReceipt", err)
		})
	}
}

func TestResumeRepositoryLoadSnapshotRejectsMissingRecordAndStaleGeneration(t *testing.T) {
	_, store := newResumeRepositoryStore(t, resumeRepositoryRecord(t, 4))
	repository := NewResumeRepository(store)
	if _, err := repository.LoadSnapshot(t.Context(), "io-absent", 4); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing err=%v", err)
	}
	for _, generation := range []uint64{0, 3, 5} {
		if _, err := repository.LoadSnapshot(t.Context(), "io-resume-repository", generation); err == nil || !strings.Contains(err.Error(), "stale lease generation: current=4") {
			t.Fatalf("generation %d err=%v", generation, err)
		}
	}
}

func TestResumeRepositoryLoadIntentRejectsMissingRecordAndPayload(t *testing.T) {
	repository, state, store := seededResumeIntent(t)

	loaded, err := repository.LoadIntent(t.Context(), state.Progress)
	if err != nil || loaded.OperationID != state.OperationID || loaded.Stage != state.Stage || string(loaded.IntentRaw) != string(state.IntentRaw) || string(loaded.RecordRaw) != string(state.RecordRaw) {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}

	noPending := state.Progress
	noPending.Execution.Pending = nil
	if _, err := repository.LoadIntent(t.Context(), noPending); err == nil || !strings.Contains(err.Error(), "pending intent is required") {
		t.Fatalf("no pending err=%v", err)
	}

	unknownRecord := state.Progress
	unknownRecord.Record.ID = "io-absent"
	if _, err := repository.LoadIntent(t.Context(), unknownRecord); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing record err=%v", err)
	}

	if err := store.Apply(t.Context(), []port.RecordMutation{{Bucket: "external_intent_v1", ID: state.OperationID, Delete: true}}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.LoadIntent(t.Context(), state.Progress); err == nil || !strings.Contains(err.Error(), "intent payload is missing") {
		t.Fatalf("missing payload err=%v", err)
	}
}

func TestResumeRepositoryBeginIntentRejectsStaleGenerationAndMissingBinding(t *testing.T) {
	record := resumeRepositoryRecord(t, 4)
	_, store := newResumeRepositoryStore(t, record)
	repository := NewResumeRepository(store)
	snapshot, err := repository.LoadSnapshot(t.Context(), record.ID, 4)
	if err != nil {
		t.Fatal(err)
	}
	artifacts := leasecontract.ResumeArtifacts{IssueBodySHA256: strings.Repeat("a", 64), OwnerPromptPath: "/worktree/prompt", OwnerPromptSHA256: strings.Repeat("c", 64), ContextPacketPath: "/worktree/packet", ContextPacketSHA256: strings.Repeat("d", 64)}

	noRaw := snapshot
	noRaw.Raw = nil
	if _, err := repository.BeginIntent(t.Context(), noRaw, artifacts, resumeRepositoryPlan(), strings.Repeat("1", 32)); err == nil {
		t.Fatal("empty raw snapshot must be rejected")
	}

	noBinding := snapshot
	stable := snapshot.Record.Stable
	execution := *stable.Execution
	execution.Orca = nil
	stable.Execution = &execution
	noBinding.Record.Stable = stable
	if _, err := repository.BeginIntent(t.Context(), noBinding, artifacts, resumeRepositoryPlan(), strings.Repeat("1", 32)); err == nil {
		t.Fatal("snapshot without Orca binding must be rejected")
	}

	advanced := resumeRepositoryRecord(t, 5)
	if err := store.Apply(t.Context(), []port.RecordMutation{resumeRepositoryMutation(t, advanced)}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.BeginIntent(t.Context(), snapshot, artifacts, resumeRepositoryPlan(), strings.Repeat("1", 32)); err == nil {
		t.Fatal("begin on a superseded generation must be rejected")
	}
	if _, ok, err := store.Get("external_intent_v1", strings.Repeat("1", 32)); err != nil || ok {
		t.Fatalf("rejected begin persisted an intent: ok=%v err=%v", ok, err)
	}
}

func TestResumeRepositoryRecordFailureRejectsStaleIntentWithoutChangingRecord(t *testing.T) {
	repository, state, store := seededResumeIntent(t)
	staleIntent(t, store, state)
	before, _, err := store.Get(recordBucket, state.Progress.Record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.RecordFailure(t.Context(), state, "unknown", errors.New("x")); err == nil || err.Error() != "stale raw intent snapshot" {
		t.Fatalf("err=%v", err)
	}
	after, _, err := store.Get(recordBucket, state.Progress.Record.ID)
	if err != nil || string(before) != string(after) {
		t.Fatalf("record changed by rejected failure: err=%v", err)
	}
}

func TestReconcileRepositoryGuardsMissingStoreAndEvidence(t *testing.T) {
	_, state, store := seededResumeIntent(t)
	intent := reconcileIntentFrom(state)
	ctx := t.Context()

	for name, repository := range map[string]*ReconcileRepository{
		"nil repository": nil,
		"nil store":      NewReconcileRepositoryWithSnapshot(nil, nil, nil, nil, nil),
	} {
		if _, err := repository.Canonicalize(ctx, "io-x"); err == nil || !strings.Contains(err.Error(), "store is required") {
			t.Fatalf("%s canonicalize err=%v", name, err)
		}
		if _, err := repository.Latest(ctx, "io-x"); err == nil || !strings.Contains(err.Error(), "store is required") {
			t.Fatalf("%s latest err=%v", name, err)
		}
		if err := repository.RecordFailure(ctx, intent, "unknown", errors.New("x")); err == nil || !strings.Contains(err.Error(), "store is required") {
			t.Fatalf("%s record failure err=%v", name, err)
		}
		if _, err := repository.ClearIntent(ctx, intent, nil); err == nil || !strings.Contains(err.Error(), "store is required") {
			t.Fatalf("%s clear err=%v", name, err)
		}
	}
	if _, err := (*ReconcileRepository)(nil).MarkInvoking(ctx, intent); err == nil || !strings.Contains(err.Error(), "store is required") {
		t.Fatalf("nil repository mark invoking err=%v", err)
	}

	plain := NewReconcileRepositoryWithSnapshot(&claimStore{}, nil, nil, nil, nil)
	if err := plain.RecordFailure(ctx, intent, "unknown", errors.New("x")); err == nil || !strings.Contains(err.Error(), "does not support raw CAS") {
		t.Fatalf("record failure err=%v", err)
	}
	if _, err := plain.ClearIntent(ctx, intent, nil); err == nil || !strings.Contains(err.Error(), "does not support raw CAS") {
		t.Fatalf("clear err=%v", err)
	}

	repository := NewReconcileRepositoryWithSnapshot(store, nil, nil, nil, nil)
	missingIntent := intent
	missingIntent.IntentRaw = nil
	if _, err := repository.ClearIntent(ctx, missingIntent, nil); err == nil || !strings.Contains(err.Error(), "raw CAS evidence is required") {
		t.Fatalf("clear without intent raw err=%v", err)
	}
	corruptIntent := intent
	corruptIntent.IntentRaw = []byte("{not json")
	if _, err := repository.ClearIntent(ctx, corruptIntent, nil); err == nil {
		t.Fatal("clear with undecodable intent must fail")
	}
}

func TestReconcileRepositoryClearIntentDistinguishesStaleRecordFromStaleIntent(t *testing.T) {
	_, state, store := seededResumeIntent(t)
	staleRecord(t, store, state)
	_, err := NewReconcileRepositoryWithSnapshot(store, nil, nil, nil, nil).ClearIntent(t.Context(), reconcileIntentFrom(state), errors.New("x"))
	if err == nil || err.Error() != "stale raw record snapshot" {
		t.Fatalf("stale record err=%v", err)
	}

	_, state, store = seededResumeIntent(t)
	staleIntent(t, store, state)
	_, err = NewReconcileRepositoryWithSnapshot(store, nil, nil, nil, nil).ClearIntent(t.Context(), reconcileIntentFrom(state), errors.New("x"))
	if err == nil || err.Error() != "stale raw intent snapshot" {
		t.Fatalf("stale intent err=%v", err)
	}
}

func TestReconcileRepositoryLatestAndCanonicalizeReportMissingRecordsAndIntent(t *testing.T) {
	_, state, store := seededResumeIntent(t)
	repository := NewReconcileRepositoryWithSnapshot(store, nil, nil, nil, nil)
	if _, err := repository.Latest(t.Context(), "io-absent"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("latest missing err=%v", err)
	}
	if _, err := repository.Canonicalize(t.Context(), "io-absent"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("canonicalize missing err=%v", err)
	}
	if err := store.Apply(t.Context(), []port.RecordMutation{{Bucket: "external_intent_v1", ID: state.OperationID, Delete: true}}); err != nil {
		t.Fatal(err)
	}
	result, err := repository.Canonicalize(t.Context(), state.Progress.Record.ID)
	if err == nil || !strings.Contains(err.Error(), "orca_intent_invalid") || result.Progress.Record.ID != state.Progress.Record.ID {
		t.Fatalf("missing intent result=%+v err=%v", result, err)
	}
}
