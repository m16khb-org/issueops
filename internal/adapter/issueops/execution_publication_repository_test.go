package issueops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	cycleapp "issueops/internal/application/issueopscycle"
	remoteapp "issueops/internal/application/issueopsremote"
	contract "issueops/internal/contract/issueops"
	publicationcontract "issueops/internal/contract/issueopspublication"
)

func TestPublicationRepositoryPreservesStoredSnapshots(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "195-publication-repository")
	operationID := "0123456789abcdef0123456789abcdef"
	fixture.record.Execution.Pending = &contract.ExternalIntent{
		OperationID: operationID, Kind: externalIntentRemotePR,
		Marker: "publication-marker", StartedAt: "2026-09-28T00:00:00Z",
	}
	record, err := writeIssueOps(stateRoot, fixture.record)
	if err != nil {
		t.Fatal(err)
	}
	recordRaw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	payload := publicationcontract.IntentPayload{
		SchemaVersion: contract.IssueOpsSchemaVersion, OperationID: operationID, Generation: 1,
		Provider: "github", Kind: "pr", InvocationState: "unknown",
		Request: publicationcontract.ProviderCreateRequest{Labels: []string{"enhancement"}},
	}
	intentRaw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, record.ID, recordRaw); err != nil {
		t.Fatal(err)
	}
	if err := db.Put(externalIntentBucket, operationID, intentRaw); err != nil {
		t.Fatal(err)
	}
	repository := newPublicationJournalForTest(stateRoot, nil, nil)
	intent, err := repository.LoadIntent(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(intent.Record.Raw, recordRaw) || !bytes.Equal(intent.Raw, intentRaw) || intent.OperationID != operationID {
		t.Fatalf("stored bytes changed: record=%q intent=%q", intent.Record.Raw, intent.Raw)
	}
	intent.Record.Raw[0], intent.Raw[0], intent.Request.Labels[0] = 'x', 'y', "changed"
	again, err := repository.LoadIntent(context.Background(), record.ID)
	if err != nil || !bytes.Equal(again.Record.Raw, recordRaw) || !bytes.Equal(again.Raw, intentRaw) || again.Request.Labels[0] != "enhancement" {
		t.Fatalf("returned snapshot changed storage: intent=%+v err=%v", again, err)
	}
	latest, err := repository.Latest(context.Background(), record.ID)
	if err != nil || !bytes.Equal(latest.Raw, recordRaw) {
		t.Fatalf("latest snapshot changed: record=%+v err=%v", latest, err)
	}
}

func newPendingPublicationFixture(t *testing.T) (string, *remoteapp.PublicationJournal, contract.IssueOpsRecord, publicationcontract.IntentPayload) {
	t.Helper()
	root := t.TempDir()
	fixture := newClaimableExecutionFixture(t, root, "196-publication-receipt")
	actor := executionActor("codex", "publication-owner")
	fixture.record.Phase = contract.IssueOpsPhasePR
	fixture.record.IssueURL = "https://github.com/example/issueops/issues/69"
	fixture.record.Execution.Lease.Status = contract.LeaseStatusActive
	fixture.record.Execution.Lease.ClaimedAt = "2026-09-28T00:00:00Z"
	fixture.record.Execution.Lease.Holder = &actor
	fixture.record.Execution.Lease.ClaimTokenSHA256 = ""
	operationID := "0123456789abcdef0123456789abcdef"
	fixture.record.Execution.Pending = &contract.ExternalIntent{
		OperationID: operationID, Kind: externalIntentRemotePR,
		Marker: "publication-marker", StartedAt: "2026-09-28T00:00:00Z",
	}
	record, err := writeIssueOps(root, fixture.record)
	if err != nil {
		t.Fatal(err)
	}
	payload := publicationcontract.IntentPayload{
		SchemaVersion: contract.IssueOpsSchemaVersion, OperationID: operationID, Generation: 1,
		Provider: "github", Kind: "pr", InvocationState: "unknown",
		Request: publicationcontract.ProviderCreateRequest{
			Labels: []string{"enhancement"}, Assignees: []string{"maintainer"}, BaseBranch: "main",
			Host: actor.Host, SessionID: actor.SessionID, CWD: fixture.worktree,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(externalIntentBucket, operationID, raw); err != nil {
		t.Fatal(err)
	}
	record, err = ReadIssueOps(root, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 9, 28, 1, 2, 3, 0, time.UTC) }
	return root, newPublicationJournalForTest(root, now, nil), record, payload
}

func TestPublicationRepositoryReceiptPreservesActiveLease(t *testing.T) {
	root, repository, before, payload := newPendingPublicationFixture(t)
	intent, err := repository.LoadIntent(context.Background(), before.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repository.Complete(context.Background(), intent, "https://github.com/example/issueops/pull/196", true)
	if err != nil {
		t.Fatal(err)
	}
	after, err := ReadIssueOps(root, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.RemoteArtifact == nil || after.RemoteArtifact.URL != "https://github.com/example/issueops/pull/196" || after.RemoteArtifact.VerifiedAt != "2026-09-28T01:02:03Z" {
		t.Fatalf("receipt not persisted: %+v", after.RemoteArtifact)
	}
	if after.Execution.Pending != nil || after.Execution.Failure != nil || !reflect.DeepEqual(after.Execution.Lease, before.Execution.Lease) || after.Phase != contract.IssueOpsPhasePR {
		t.Fatalf("publication changed execution lifecycle: %+v", after.Execution)
	}
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists, err := db.Get(externalIntentBucket, payload.OperationID); err != nil || exists {
		t.Fatalf("intent remains after receipt: exists=%v err=%v", exists, err)
	}
	stored, exists, err := db.Get(issueOpsBucket, before.ID)
	if err != nil || !exists || !bytes.Equal(snapshot.Raw, stored) {
		t.Fatalf("receipt snapshot differs from storage: exists=%v err=%v", exists, err)
	}
}

func TestPublicationRepositoryRejectsStaleReceiptWithoutWrites(t *testing.T) {
	for _, mutation := range []string{"generation", "payload"} {
		t.Run(mutation, func(t *testing.T) {
			root, repository, record, payload := newPendingPublicationFixture(t)
			intent, err := repository.LoadIntent(context.Background(), record.ID)
			if err != nil {
				t.Fatal(err)
			}
			db, err := sqlstore.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			wantError := "stale execution generation"
			if mutation == "generation" {
				record.Execution.Lease.Generation++
				if _, err := writeIssueOps(root, record); err != nil {
					t.Fatal(err)
				}
			} else {
				payload.RetryCount++
				raw, err := json.Marshal(payload)
				if err != nil {
					t.Fatal(err)
				}
				if err := db.Put(externalIntentBucket, payload.OperationID, raw); err != nil {
					t.Fatal(err)
				}
				wantError = "payload changed before remote receipt CAS"
			}
			beforeRecord, _, err := db.Get(issueOpsBucket, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			beforeIntent, _, err := db.Get(externalIntentBucket, payload.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = repository.Complete(context.Background(), intent, "https://github.com/example/issueops/pull/196", true)
			if err == nil || !strings.Contains(err.Error(), wantError) {
				t.Fatalf("stale receipt accepted: %v", err)
			}
			afterRecord, _, err := db.Get(issueOpsBucket, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			afterIntent, _, err := db.Get(externalIntentBucket, payload.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(beforeRecord, afterRecord) || !bytes.Equal(beforeIntent, afterIntent) {
				t.Fatal("rejected receipt modified storage")
			}
		})
	}
}

func TestPublicationRepositoryRejectsInvalidArtifactWithoutWrites(t *testing.T) {
	for _, tc := range []struct{ name, url, assignee, want string }{
		{"wrong project", "https://github.com/other/project/pull/196", "maintainer", "match linked issue project"},
		{"placeholder assignee", "https://github.com/example/issueops/pull/196", "@me", "not placeholder"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, repository, record, payload := newPendingPublicationFixture(t)
			payload.Request.Assignees = []string{tc.assignee}
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			db, err := sqlstore.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Put(externalIntentBucket, payload.OperationID, raw); err != nil {
				t.Fatal(err)
			}
			intent, err := repository.LoadIntent(context.Background(), record.ID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = repository.Complete(context.Background(), intent, tc.url, true)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid artifact accepted: %v", err)
			}
			storedRecord, _, err := db.Get(issueOpsBucket, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			storedIntent, _, err := db.Get(externalIntentBucket, payload.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(intent.Record.Raw, storedRecord) || !bytes.Equal(raw, storedIntent) {
				t.Fatal("rejected artifact modified storage")
			}
		})
	}
}

func newPublicationJournalForTest(root string, now func() time.Time, operationID func() (string, error)) *remoteapp.PublicationJournal {
	return remoteapp.NewPublicationJournal(RemotePublicationStore{StateRoot: root}, RemotePublicationObserver{StateRoot: root, Clock: now, OperationIDFactory: operationID}, cycleapp.NewMutationAuthority(samePath))
}

func TestPublicationJournalRechecksHolderBeforeIntentWrite(t *testing.T) {
	root, journal, record, _ := newPendingPublicationFixture(t)
	record.Execution.Pending = nil
	if _, err := writeIssueOps(root, record); err != nil {
		t.Fatal(err)
	}
	before, err := journal.Latest(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	actor := record.Execution.Lease.Holder
	prepared := publicationcontract.PreparedCreate{
		Command: publicationcontract.CreateCommand{ID: record.ID, Provider: "github", ExpectedGeneration: record.Execution.Lease.Generation, CWD: record.Execution.Workspace.Root,
			Actor: publicationcontract.Actor{Host: actor.Host, SessionID: "different-holder", AgentID: actor.AgentID}},
		Eligibility: publicationcontract.CreateEligibility{Kind: "pr"},
	}
	for _, receipt := range actor.ProcessAncestry {
		prepared.Command.Actor.ProcessAncestry = append(prepared.Command.Actor.ProcessAncestry, publicationcontract.ProcessReceipt(receipt))
	}
	_, err = journal.BeginCreate(context.Background(), prepared)
	if err == nil || !strings.Contains(err.Error(), "holder") {
		t.Fatalf("error=%v", err)
	}
	after, err := journal.Latest(context.Background(), record.ID)
	if err != nil || !bytes.Equal(before.Raw, after.Raw) {
		t.Fatalf("rejected intent changed record: %v", err)
	}
}

func TestPublicationJournalRejectsAbsentOrOtherPendingIntent(t *testing.T) {
	for _, mode := range []string{"unprepared", "no-intent", "other-kind"} {
		t.Run(mode, func(t *testing.T) {
			root, journal, record, _ := newPendingPublicationFixture(t)
			switch mode {
			case "unprepared":
				record.Execution = nil
			case "no-intent":
				record.Execution.Pending = nil
			case "other-kind":
				record.Execution.Pending.Kind = "orca"
			}
			if _, err := writeIssueOps(root, record); err != nil {
				t.Fatal(err)
			}
			before, err := journal.Latest(context.Background(), record.ID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = journal.LoadIntent(context.Background(), record.ID)
			if err == nil || err.Error() != "remote publication intent is not pending" {
				t.Fatalf("error=%v", err)
			}
			after, err := journal.Latest(context.Background(), record.ID)
			if err != nil || !bytes.Equal(before.Raw, after.Raw) {
				t.Fatalf("rejected load changed record: %v", err)
			}
		})
	}
}

func TestPublicationJournalRetryAndTerminalFailureAreAtomic(t *testing.T) {
	root, journal, record, payload := newPendingPublicationFixture(t)
	original, err := journal.LoadIntent(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	retried, err := journal.MarkRetry(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	if retried.RetryCount != 1 || retried.InvocationState != publicationcontract.InvocationUnknown {
		t.Fatalf("retry=%+v", retried)
	}
	if _, err := journal.MarkRetry(context.Background(), original); err == nil || !strings.Contains(err.Error(), "payload changed before retry CAS") {
		t.Fatalf("stale retry accepted: %v", err)
	}
	if _, err := journal.CompleteNotInvoked(context.Background(), original, errors.New("failed")); err == nil || !strings.Contains(err.Error(), "payload changed before terminal pre-invocation receipt") {
		t.Fatalf("stale terminal receipt accepted: %v", err)
	}
	current, err := journal.LoadIntent(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current.Raw, retried.Raw) || !bytes.Equal(current.Record.Raw, retried.Record.Raw) {
		t.Fatal("failed CAS changed stored state")
	}
	knownURL := "https://github.com/example/issueops/pull/196"
	if err := journal.RecordFailure(context.Background(), retried, publicationcontract.InvocationUnknown, " "+knownURL+" ", errors.New(strings.Repeat("x", 5000))); err != nil {
		t.Fatal(err)
	}
	current, err = journal.LoadIntent(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := ReadIssueOps(root, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.KnownURL != knownURL || current.RetryCount != 1 || observed.Execution.Pending == nil || observed.Execution.Failure.Code != "external_operation_ambiguous" || len(observed.Execution.Failure.Message) != 4096 {
		t.Fatalf("ambiguous failure lost evidence: intent=%+v failure=%+v", current, observed.Execution.Failure)
	}
	if err := journal.RecordFailure(context.Background(), current, publicationcontract.InvocationUnknown, "", nil); err != nil {
		t.Fatal(err)
	}
	current, err = journal.LoadIntent(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.KnownURL != knownURL {
		t.Fatal("empty observation cleared known URL")
	}
	snapshot, err := journal.CompleteNotInvoked(context.Background(), current, errors.New("not invoked"))
	if err != nil {
		t.Fatal(err)
	}
	observed, err = ReadIssueOps(root, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if observed.Execution.Pending != nil || observed.Execution.Failure.Code != "external_operation_not_invoked" || observed.Execution.Failure.At != "2026-09-28T01:02:03Z" || !reflect.DeepEqual(observed.Execution.Lease, record.Execution.Lease) {
		t.Fatalf("terminal failure changed execution: %+v", observed.Execution)
	}
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists, err := db.Get(externalIntentBucket, payload.OperationID); err != nil || exists {
		t.Fatalf("terminal intent remains: %v %v", exists, err)
	}
	raw, exists, err := db.Get(issueOpsBucket, record.ID)
	if err != nil || !exists || !bytes.Equal(raw, snapshot.Raw) {
		t.Fatal("terminal snapshot differs from committed record")
	}
}
