package issueops

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	contract "issueops/internal/contract/issueops"
	"issueops/internal/port"
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
	payload := externalRemotePRPayload{
		SchemaVersion: contract.IssueOpsSchemaVersion, OperationID: operationID, Generation: 1,
		Provider: "github", Kind: "pr", InvocationState: remoteInvocationUnknown,
		Request: port.IssueProviderCreatePullRequestRequest{Labels: []string{"enhancement"}},
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
	repository := NewRemotePublicationRepository(stateRoot, nil, nil)
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

func newPendingPublicationFixture(t *testing.T) (string, *RemotePublicationRepository, contract.IssueOpsRecord, externalRemotePRPayload) {
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
	payload := externalRemotePRPayload{
		SchemaVersion: contract.IssueOpsSchemaVersion, OperationID: operationID, Generation: 1,
		Provider: "github", Kind: "pr", InvocationState: remoteInvocationUnknown,
		Request: port.IssueProviderCreatePullRequestRequest{
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
	return root, NewRemotePublicationRepository(root, now, nil), record, payload
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
