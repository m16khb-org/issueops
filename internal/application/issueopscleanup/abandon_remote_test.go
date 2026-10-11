package issueopscleanup

import (
	"context"
	"errors"
	"slices"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type recordingBodyReader struct {
	port.IssueProvider
	requests []port.IssueProviderArtifactBodyRequest
	states   map[string]string
	errs     map[string]error
}

func (r *recordingBodyReader) ReadArtifactBody(_ context.Context, req port.IssueProviderArtifactBodyRequest) (port.IssueProviderArtifactBody, error) {
	r.requests = append(r.requests, req)
	return port.IssueProviderArtifactBody{State: r.states[req.URL]}, r.errs[req.URL]
}

type remoteRefCall struct{ repo, branch string }

func remoteRefFake(oid string, err error, calls *[]remoteRefCall) func(context.Context, string, string) (string, error) {
	return func(_ context.Context, repo, branch string) (string, error) {
		*calls = append(*calls, remoteRefCall{repo, branch})
		return oid, err
	}
}

func remoteAbandonRecordFixture() model.IssueOpsRecord {
	return model.IssueOpsRecord{
		ID: "cycle", Repo: "/repo", IssueURL: " https://example.test/issues/1 ",
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Kind: " pr ", URL: " https://example.test/pull/2 "},
		BranchPrepare:  &model.IssueOpsBranchPrepare{BaseBranch: "main"},
	}
}

var allRemoteFlags = model.CleanupAbandonRequest{ClosePR: true, CloseIssue: true, DeleteRemoteBranch: true}

func TestAbandonRemoteObserveWithoutFlagsNeverTouchesRemote(t *testing.T) {
	var calls []remoteRefCall
	reader := &recordingBodyReader{}
	inventory, observed := (AbandonRemoteObserver{RemoteRef: remoteRefFake("oid", nil, &calls)}).Observe(t.Context(), remoteAbandonRecordFixture(), model.CleanupAbandonRequest{}, reader, model.CleanupAbandonInventory{Branch: "feature"})
	if len(reader.requests) != 0 || len(calls) != 0 || len(observed.RemoteEffects) != 0 || len(observed.RemoteMissing) != 0 || inventory.RemoteBranchOID != "" {
		t.Fatalf("remote touched: reads=%v refs=%v observed=%+v inventory=%+v", reader.requests, calls, observed, inventory)
	}
}

func TestAbandonRemoteObservePlansEveryRequestedEffectFromReadbacks(t *testing.T) {
	var calls []remoteRefCall
	reader := &recordingBodyReader{states: map[string]string{"https://example.test/pull/2": "OPEN", "https://example.test/issues/1": "OPEN"}}
	inventory, observed := (AbandonRemoteObserver{RemoteRef: remoteRefFake("abc123", nil, &calls)}).Observe(t.Context(), remoteAbandonRecordFixture(), allRemoteFlags, reader, model.CleanupAbandonInventory{Branch: "feature"})
	if !slices.Equal(observed.RemoteEffects, []string{"close_pr", "close_issue", "remote_branch_delete"}) || len(observed.RemoteMissing) != 0 {
		t.Fatalf("observed=%+v", observed)
	}
	if observed.RemoteArtifactState != "OPEN" || observed.IssueState != "OPEN" || inventory.RemoteBranchOID != "abc123" {
		t.Fatalf("observed=%+v inventory=%+v", observed, inventory)
	}
	if !inventory.ClosePR || !inventory.CloseIssue || !inventory.DeleteRemoteBranch {
		t.Fatalf("inventory does not record the requested flags: %+v", inventory)
	}
	wantReads := []port.IssueProviderArtifactBodyRequest{
		{Repo: "/repo", Kind: "pr", URL: "https://example.test/pull/2"},
		{Repo: "/repo", Kind: "issue", URL: "https://example.test/issues/1"},
	}
	if !slices.Equal(reader.requests, wantReads) {
		t.Fatalf("reads=%+v want %+v", reader.requests, wantReads)
	}
	if len(calls) != 1 || calls[0] != (remoteRefCall{"/repo", "feature"}) {
		t.Fatalf("remote ref calls=%v", calls)
	}
}

func TestAbandonRemoteObserveDefaultsArtifactKindToIssue(t *testing.T) {
	record := remoteAbandonRecordFixture()
	record.RemoteArtifact.Kind = "  "
	reader := &recordingBodyReader{}
	(AbandonRemoteObserver{}).Observe(t.Context(), record, model.CleanupAbandonRequest{ClosePR: true}, reader, model.CleanupAbandonInventory{})
	if len(reader.requests) != 1 || reader.requests[0].Kind != "issue" {
		t.Fatalf("reads=%+v", reader.requests)
	}
}

func TestAbandonRemoteObserveRecognizesAlreadyClosedAndAbsentRemoteState(t *testing.T) {
	var calls []remoteRefCall
	reader := &recordingBodyReader{states: map[string]string{"https://example.test/pull/2": " Closed ", "https://example.test/issues/1": "CLOSED"}}
	inventory, observed := (AbandonRemoteObserver{RemoteRef: remoteRefFake("", nil, &calls)}).Observe(t.Context(), remoteAbandonRecordFixture(), allRemoteFlags, reader, model.CleanupAbandonInventory{Branch: "feature"})
	want := []string{"close_pr:already_closed", "close_issue:already_closed", "remote_branch_delete:absent"}
	if !slices.Equal(observed.RemoteEffects, want) || inventory.RemoteBranchOID != "" {
		t.Fatalf("effects=%v inventory=%+v", observed.RemoteEffects, inventory)
	}
}

func TestAbandonRemoteObserveRefusesMergedArtifactAndWithholdsAllEffects(t *testing.T) {
	var calls []remoteRefCall
	reader := &recordingBodyReader{states: map[string]string{"https://example.test/pull/2": "MERGED", "https://example.test/issues/1": "OPEN"}}
	_, observed := (AbandonRemoteObserver{RemoteRef: remoteRefFake("abc", nil, &calls)}).Observe(t.Context(), remoteAbandonRecordFixture(), allRemoteFlags, reader, model.CleanupAbandonInventory{Branch: "feature"})
	if !slices.Equal(observed.RemoteMissing, []string{"remote_artifact_unmerged"}) || len(observed.RemoteEffects) != 0 {
		t.Fatalf("observed=%+v", observed)
	}
	if len(calls) != 1 || len(reader.requests) != 2 {
		t.Fatalf("one refused effect must not hide the remaining observations: reads=%d refs=%d", len(reader.requests), len(calls))
	}
}

func TestAbandonRemoteObserveReportsUnreadableRemoteState(t *testing.T) {
	failure := errors.New("network")
	var calls []remoteRefCall
	reader := &recordingBodyReader{errs: map[string]error{"https://example.test/pull/2": failure, "https://example.test/issues/1": failure}}
	_, observed := (AbandonRemoteObserver{RemoteRef: remoteRefFake("", failure, &calls)}).Observe(t.Context(), remoteAbandonRecordFixture(), allRemoteFlags, reader, model.CleanupAbandonInventory{Branch: "feature"})
	want := []string{"remote_artifact_readable", "issue_readable", "remote_branch_readable"}
	if !slices.Equal(observed.RemoteMissing, want) || len(observed.RemoteEffects) != 0 {
		t.Fatalf("observed=%+v", observed)
	}
}

func TestAbandonRemoteObserveRefusesWhenProviderCannotReadArtifacts(t *testing.T) {
	record := remoteAbandonRecordFixture()
	req := model.CleanupAbandonRequest{ClosePR: true, CloseIssue: true}
	t.Run("no provider", func(t *testing.T) {
		_, observed := (AbandonRemoteObserver{}).Observe(t.Context(), record, req, nil, model.CleanupAbandonInventory{})
		if !slices.Equal(observed.RemoteMissing, []string{"remote_provider_unavailable", "remote_provider_unavailable"}) {
			t.Fatalf("missing=%v", observed.RemoteMissing)
		}
	})
	t.Run("provider without body reader", func(t *testing.T) {
		var bare struct{ port.IssueProvider }
		_, observed := (AbandonRemoteObserver{}).Observe(t.Context(), record, req, bare, model.CleanupAbandonInventory{})
		if !slices.Equal(observed.RemoteMissing, []string{"remote_artifact_readable", "issue_readable"}) {
			t.Fatalf("missing=%v", observed.RemoteMissing)
		}
	})
}

func TestAbandonRemoteObserveRefusesUnsatisfiablePrerequisites(t *testing.T) {
	var calls []remoteRefCall
	reader := &recordingBodyReader{}
	observer := AbandonRemoteObserver{RemoteRef: remoteRefFake("abc", nil, &calls)}
	noRemote := model.IssueOpsRecord{ID: "cycle", Repo: "/repo", BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: "main"}}
	_, observed := observer.Observe(t.Context(), noRemote, allRemoteFlags, reader, model.CleanupAbandonInventory{})
	if want := []string{"remote_artifact_required", "issue_url_required", "branch_recorded"}; !slices.Equal(observed.RemoteMissing, want) {
		t.Fatalf("missing=%v want %v", observed.RemoteMissing, want)
	}
	_, observed = observer.Observe(t.Context(), noRemote, model.CleanupAbandonRequest{DeleteRemoteBranch: true}, reader, model.CleanupAbandonInventory{Branch: "main"})
	if !slices.Equal(observed.RemoteMissing, []string{"branch_not_base"}) {
		t.Fatalf("missing=%v", observed.RemoteMissing)
	}
	if len(calls) != 0 || len(reader.requests) != 0 {
		t.Fatalf("unsatisfiable prerequisites still hit the remote: refs=%v reads=%v", calls, reader.requests)
	}
}
