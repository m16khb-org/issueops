package issueopsremote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
)

// memoryPublicationStore keeps the record and its single intent payload in
// memory. Persist mirrors the durable store's contract: RequireAbsent fails on
// an existing payload, Delete drops it, and a failed transaction rolls back.
type memoryPublicationStore struct {
	persists int
	record   model.IssueOpsRecord
	payload  map[string]contract.IntentPayload
}

func (s *memoryPublicationStore) WithinTransaction(ctx context.Context, _ string, fn func(context.Context) error) error {
	saved, payloads := s.record, map[string]contract.IntentPayload{}
	for k, v := range s.payload {
		payloads[k] = v
	}
	if err := fn(ctx); err != nil {
		s.record, s.payload = saved, payloads
		return err
	}
	return nil
}
func (s *memoryPublicationStore) Read(context.Context, string) (model.IssueOpsRecord, error) {
	return s.record, nil
}
func (s *memoryPublicationStore) ReadPayload(_ context.Context, op string) (contract.IntentPayload, error) {
	p, ok := s.payload[op]
	if !ok {
		return contract.IntentPayload{}, fmt.Errorf("payload %s not found", op)
	}
	return p, nil
}
func (s *memoryPublicationStore) RecordSnapshot(context.Context, string) (contract.RecordSnapshot, error) {
	raw, err := json.Marshal(s.record)
	return contract.RecordSnapshot{ID: s.record.ID, Raw: raw}, err
}
func (s *memoryPublicationStore) PayloadRaw(_ context.Context, op string) ([]byte, error) {
	return json.Marshal(s.payload[op])
}
func (s *memoryPublicationStore) DecodeSnapshot(intent contract.Intent) (model.IssueOpsRecord, contract.IntentPayload, error) {
	var record model.IssueOpsRecord
	var payload contract.IntentPayload
	if err := json.Unmarshal(intent.Record.Raw, &record); err != nil {
		return record, payload, err
	}
	return record, payload, json.Unmarshal(intent.Raw, &payload)
}
func (s *memoryPublicationStore) Persist(_ context.Context, record model.IssueOpsRecord, mutation contract.IntentMutation) (model.IssueOpsRecord, error) {
	s.persists++
	if mutation.RequireAbsent {
		if _, exists := s.payload[mutation.OperationID]; exists {
			return model.IssueOpsRecord{}, errors.New("intent already exists")
		}
	}
	s.record = record
	switch {
	case mutation.Delete:
		delete(s.payload, mutation.OperationID)
	case mutation.Payload != nil:
		s.payload[mutation.OperationID] = *mutation.Payload
	}
	return record, nil
}

type journalEnvironment struct{ opID string }

func (e journalEnvironment) PathsMatch(a, b string) bool { return a == b }
func (journalEnvironment) Timestamp() string             { return "2026-01-01T00:00:00Z" }
func (e journalEnvironment) NewOperationID() (string, error) {
	return e.opID, nil
}

type journalAuthority struct{ err error }

func (a journalAuthority) Authorize(context.Context, model.IssueOpsRecord, model.IssueOpsActor) error {
	return a.err
}

const journalOp = "0123456789abcdef0123456789abcdef"

func journalRecord() model.IssueOpsRecord {
	holder := model.NativeActor{Host: "codex", SessionID: "s1", AgentID: "a1"}
	return model.IssueOpsRecord{
		ID: "cycle", Phase: model.IssueOpsPhasePR, IssueURL: "https://github.com/acme/app/issues/7",
		Execution: &model.Execution{
			Workspace: model.Workspace{Root: "/wt"},
			Lease:     model.WriteLease{Generation: 2, Status: model.LeaseStatusActive, Holder: &holder},
		},
	}
}

func journalFixture() (*PublicationJournal, *memoryPublicationStore, contract.PreparedCreate) {
	store := &memoryPublicationStore{record: journalRecord(), payload: map[string]contract.IntentPayload{}}
	journal := NewPublicationJournal(store, journalEnvironment{opID: journalOp}, journalAuthority{})
	prepared := contract.PreparedCreate{
		Command: contract.CreateCommand{ID: "cycle", Provider: "github", ExpectedGeneration: 2, CWD: "/wt", Confirm: true,
			Actor: contract.Actor{Host: "codex", SessionID: "s1", AgentID: "a1"}},
		Request:     contract.ProviderCreateRequest{Title: "t", Body: " body ", HeadBranch: "7-x", BaseBranch: "main", Labels: []string{"bug"}, Assignees: []string{"octocat"}, Confirm: true, Host: "codex", SessionID: "s1", AgentID: "a1", CWD: "/wt"},
		Eligibility: contract.CreateEligibility{Provider: "github", Kind: "pr"},
	}
	return journal, store, prepared
}

func TestPublicationJournalBeginCreateSealsMarkerAndPendingIntent(t *testing.T) {
	journal, store, prepared := journalFixture()
	intent, err := journal.BeginCreate(t.Context(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	marker := "<!-- issueops:issueops-v1 operation=" + journalOp + " -->"
	if !strings.HasSuffix(intent.Request.Body, "\n\n"+marker) || !strings.HasPrefix(intent.Request.Body, "body\n") {
		t.Fatalf("body=%q", intent.Request.Body)
	}
	if intent.OperationID != journalOp || intent.Generation != 2 || intent.InvocationState != contract.InvocationUnknown || intent.Kind != "pr" {
		t.Fatalf("intent=%+v", intent)
	}
	pending := store.record.Execution.Pending
	if pending == nil || pending.OperationID != journalOp || pending.Kind != contract.RemoteIntentKind || pending.Marker != marker {
		t.Fatalf("pending=%+v", pending)
	}
	if prepared.Request.Body != " body " {
		t.Fatal("caller's prepared request must not be mutated")
	}
	// A second begin must fail closed while the first intent is pending.
	if _, err := journal.BeginCreate(t.Context(), prepared); err == nil || !strings.Contains(err.Error(), "authority changed") {
		t.Fatalf("second begin err=%v", err)
	}
}

func TestPublicationJournalBeginCreateRejectsBeforeAnyWrite(t *testing.T) {
	boom := errors.New("denied")
	cases := []struct {
		name   string
		mutate func(*PublicationJournal, *memoryPublicationStore, *contract.PreparedCreate)
		is     error
		msg    string
	}{
		{"bad operation id", func(j *PublicationJournal, _ *memoryPublicationStore, _ *contract.PreparedCreate) {
			j.environment = journalEnvironment{opID: "XYZ"}
		}, nil, "32 lowercase hexadecimal"},
		{"authority denied", func(j *PublicationJournal, _ *memoryPublicationStore, _ *contract.PreparedCreate) {
			j.authority = journalAuthority{err: boom}
		}, boom, ""},
		{"stale generation", func(_ *PublicationJournal, _ *memoryPublicationStore, p *contract.PreparedCreate) {
			p.Command.ExpectedGeneration = 1
		}, nil, "stale lease generation"},
		{"not prepared", func(_ *PublicationJournal, s *memoryPublicationStore, _ *contract.PreparedCreate) {
			s.record.Execution = nil
		}, nil, "authority changed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			journal, store, prepared := journalFixture()
			tc.mutate(journal, store, &prepared)
			before := store.record.Execution
			_, err := journal.BeginCreate(t.Context(), prepared)
			if err == nil || (tc.is != nil && !errors.Is(err, tc.is)) || (tc.msg != "" && !strings.Contains(err.Error(), tc.msg)) {
				t.Fatalf("err=%v", err)
			}
			if store.persists != 0 || len(store.payload) != 0 || store.record.Execution != before {
				t.Fatal("rejected begin must not persist anything")
			}
		})
	}
}

func TestPublicationJournalRetryAndFailureCarryCountersAndKnownURL(t *testing.T) {
	journal, store, prepared := journalFixture()
	intent, err := journal.BeginCreate(t.Context(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	retried, err := journal.MarkRetry(t.Context(), intent)
	if err != nil || retried.RetryCount != 1 || retried.InvocationState != contract.InvocationUnknown {
		t.Fatalf("retried=%+v err=%v", retried, err)
	}
	if _, err := journal.MarkRetry(t.Context(), intent); err == nil || !strings.Contains(err.Error(), "payload changed before") {
		t.Fatalf("stale intent snapshot must not retry again: %v", err)
	}

	err = journal.RecordFailure(t.Context(), retried, contract.InvocationNotInvokedProven, " https://github.com/acme/app/pull/3 ", errors.New("gh failed"))
	if err != nil {
		t.Fatal(err)
	}
	stored := store.payload[journalOp]
	if stored.KnownURL != "https://github.com/acme/app/pull/3" || stored.RetryCount != 1 || stored.InvocationState != string(contract.InvocationNotInvokedProven) {
		t.Fatalf("stored=%+v", stored)
	}
	failure := store.record.Execution.Failure
	if failure == nil || failure.Code != "external_operation_ambiguous" || failure.Message != "gh failed" || store.record.Execution.Pending == nil {
		t.Fatalf("failure=%+v", failure)
	}
}

func TestPublicationJournalLoadIntentRebuildsEligibilityFromRecord(t *testing.T) {
	journal, _, prepared := journalFixture()
	if _, err := journal.LoadIntent(t.Context(), "cycle"); err == nil || err.Error() != "remote publication intent is not pending" {
		t.Fatalf("no pending intent: %v", err)
	}
	begun, err := journal.BeginCreate(t.Context(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := journal.LoadIntent(t.Context(), "cycle")
	if err != nil {
		t.Fatal(err)
	}
	want := contract.CreateEligibility{Provider: "github", Kind: "pr", Confirm: true, PhasePR: true, ExecutionActive: true, NoPending: false, NoArtifact: true, BranchAuthority: true, CanonicalLabelsAssignees: true}
	if loaded.Eligibility != want || loaded.OperationID != begun.OperationID || loaded.Request.Body != begun.Request.Body {
		t.Fatalf("loaded=%+v", loaded)
	}
	latest, err := journal.Latest(t.Context(), "cycle")
	if err != nil || latest.ID != "cycle" || len(latest.Raw) == 0 {
		t.Fatalf("latest=%+v err=%v", latest, err)
	}
}

func TestPublicationJournalCompleteSealsArtifactAndClearsIntent(t *testing.T) {
	journal, store, prepared := journalFixture()
	intent, err := journal.BeginCreate(t.Context(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := journal.Complete(t.Context(), intent, "https://github.com/acme/app/pull/3", true); err != nil {
		t.Fatal(err)
	}
	artifact := store.record.RemoteArtifact
	if artifact == nil || artifact.URL != "https://github.com/acme/app/pull/3" || artifact.Provider != "github" || artifact.Kind != "pr" || artifact.TargetBranch != "main" || artifact.VerifiedAt == "" {
		t.Fatalf("artifact=%+v", artifact)
	}
	if store.record.Execution.Pending != nil || len(store.payload) != 0 {
		t.Fatal("completion must clear the pending intent and its payload")
	}
}

func TestPublicationJournalCompleteEnforcesOriginalLeaseOnlyWhenAsked(t *testing.T) {
	journal, store, prepared := journalFixture()
	intent, err := journal.BeginCreate(t.Context(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	store.record.Execution.Lease.Generation = 3 // lease replaced after the intent was sealed
	_, err = journal.Complete(t.Context(), intent, "https://github.com/acme/app/pull/3", true)
	if err == nil || !strings.Contains(err.Error(), "stale execution generation") || store.record.RemoteArtifact != nil {
		t.Fatalf("stale lease must not seal a receipt: %v", err)
	}
	// Reconcile recovery completes without the original lease.
	if _, err := journal.Complete(t.Context(), intent, "https://github.com/acme/app/pull/3", false); err != nil {
		t.Fatalf("recovery completion: %v", err)
	}
	if store.record.RemoteArtifact == nil {
		t.Fatal("recovery must seal the artifact")
	}
}

func TestPublicationJournalCompleteRejectsForeignProviderURL(t *testing.T) {
	journal, store, prepared := journalFixture()
	intent, err := journal.BeginCreate(t.Context(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := journal.Complete(t.Context(), intent, "https://github.com/acme/app/issues/3", true); err == nil {
		t.Fatal("an issue URL is not a pull request artifact")
	}
	if store.persists != 1 || store.record.RemoteArtifact != nil || store.record.Execution.Pending == nil {
		t.Fatal("rejected receipt must roll back and not persist (only the begin persisted)")
	}
}

func TestPublicationJournalCompleteNotInvokedClearsPendingAndRecordsFailure(t *testing.T) {
	journal, store, prepared := journalFixture()
	intent, err := journal.BeginCreate(t.Context(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := journal.CompleteNotInvoked(t.Context(), intent, errors.New("gh missing")); err != nil {
		t.Fatal(err)
	}
	exec := store.record.Execution
	if exec.Pending != nil || exec.Failure == nil || exec.Failure.Code != "external_operation_not_invoked" || len(store.payload) != 0 {
		t.Fatalf("exec=%+v payload=%v", exec, store.payload)
	}
	// Replaying against the cleared intent fails closed.
	if _, err := journal.CompleteNotInvoked(t.Context(), intent, nil); err == nil || !strings.Contains(err.Error(), "external intent changed before") {
		t.Fatalf("replay err=%v", err)
	}
}
