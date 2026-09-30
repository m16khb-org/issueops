package issueopsexecution

import (
	"context"
	"errors"
	"reflect"
	"testing"

	snapshot "issueops/internal/contract/executionissue"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestServiceRejectsSnapshotActionBeforeObservation(t *testing.T) {
	service := Service{ReadRecord: func(string, string) (model.IssueOpsRecord, error) {
		t.Fatal("invalid action read record")
		return model.IssueOpsRecord{}, nil
	}}
	got, err := service.Execute(t.Context(), "state", model.ExecutionActionRequest{ID: "cycle", Action: model.ExecutionActionStatus, IssueSnapshot: &snapshot.ExecutionIssueSnapshotEvidence{}}, port.ExecutionActionDependencies{})
	if got != nil || err == nil || err.Error() != `issue_snapshot is not supported for execution action "status"` {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestServiceStatusWithoutSnapshotDoesNotReadRecord(t *testing.T) {
	calls := 0
	service := Service{ReadRecord: func(string, string) (model.IssueOpsRecord, error) {
		t.Fatal("status read record")
		return model.IssueOpsRecord{}, nil
	}}
	got, err := service.Execute(t.Context(), "state", model.ExecutionActionRequest{ID: "cycle", Action: model.ExecutionActionStatus}, port.ExecutionActionDependencies{Status: func(_ context.Context, root, id string) (model.ExecutionResult, error) {
		calls++
		if root != "state" || id != "cycle" {
			t.Fatalf("scope=%s %s", root, id)
		}
		return model.ExecutionResult{OK: true, ID: id}, nil
	}})
	if err != nil || calls != 1 || !reflect.DeepEqual(got, model.ExecutionResult{OK: true, ID: "cycle"}) {
		t.Fatalf("got=%v err=%v calls=%d", got, err, calls)
	}
}

func TestServiceSnapshotReaderBindsRecordAndCancellation(t *testing.T) {
	const url = "https://gitlab.com/acme/project/-/issues/7"
	reads := 0
	var paths [][2]string
	service := Service{ReadRecord: func(root, id string) (model.IssueOpsRecord, error) {
		reads++
		if root != "state" || id != "cycle" {
			t.Fatalf("scope=%s %s", root, id)
		}
		return model.IssueOpsRecord{Repo: "/repo", IssueURL: url, BranchPrepare: &model.IssueOpsBranchPrepare{Provider: "gitlab", IssueURL: url}}, nil
	}, SamePath: func(a, b string) bool { paths = append(paths, [2]string{a, b}); return a == b }}
	req := model.ExecutionActionRequest{ID: "cycle", Action: model.ExecutionActionClaim, IssueSnapshot: &snapshot.ExecutionIssueSnapshotEvidence{Provider: "gitlab", Source: "glab_mcp", WebURL: "https://gitlab.com/acme/project/-/work_items/7", Body: "body", State: "opened"}}
	reader, source, err := service.snapshotReader("state", req, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.IssueSnapshot.Body = "changed"
	got, err := reader(t.Context(), "gitlab", snapshot.ExecutionIssueSnapshotRequest{Repo: "/repo", URL: url})
	if err != nil || got.Body != "body" || got.URL != url || source() != "glab_mcp" || reads != 1 {
		t.Fatalf("got=%+v err=%v reads=%d source=%s", got, err, reads, source())
	}
	if len(paths) != 1 || paths[0] != [2]string{"/repo", "/repo"} {
		t.Fatalf("paths=%v", paths)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = reader(ctx, "gitlab", snapshot.ExecutionIssueSnapshotRequest{Repo: "/repo", URL: url})
	if !errors.Is(err, context.Canceled) || len(paths) != 1 {
		t.Fatalf("cancel err=%v paths=%v", err, paths)
	}
	_, err = reader(t.Context(), "gitlab", snapshot.ExecutionIssueSnapshotRequest{Repo: "/other", URL: url})
	if err == nil {
		t.Fatal("unbound repo accepted")
	}
}

func TestFallbackSourceIsInvocationLocalAndFailureDoesNotDecorate(t *testing.T) {
	const url = "https://gitlab.com/acme/project/-/issues/7"
	fallback := func(context.Context, string, snapshot.ExecutionIssueSnapshotRequest) (snapshot.ExecutionIssueSnapshot, error) {
		return snapshot.ExecutionIssueSnapshot{URL: url, Body: "body", State: "opened"}, nil
	}
	a, as, _ := executionGitLabFallbackSnapshotReader(fallback)
	_, bs, _ := executionGitLabFallbackSnapshotReader(fallback)
	if _, err := a(t.Context(), "gitlab", snapshot.ExecutionIssueSnapshotRequest{URL: url}); err != nil {
		t.Fatal(err)
	}
	if as() != "glab_cli" || bs() != "" {
		t.Fatalf("sources=%q %q", as(), bs())
	}
	failure := errors.New("handler failed")
	service := Service{}
	got, err := service.Execute(t.Context(), "state", model.ExecutionActionRequest{ID: "cycle", Action: model.ExecutionActionClaim}, port.ExecutionActionDependencies{ReadIssue: fallback, Claim: func(ctx context.Context, _ string, _ model.ExecutionClaimRequest, deps model.ExecutionClaimDependencies) (model.ExecutionResult, error) {
		if _, err := deps.ReadIssue(ctx, "gitlab", snapshot.ExecutionIssueSnapshotRequest{URL: url}); err != nil {
			t.Fatal(err)
		}
		return model.ExecutionResult{ID: "cycle"}, failure
	}})
	if !errors.Is(err, failure) || got.(model.ExecutionResult).IssueSnapshotSource != "" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestReconcileRejectsAmbiguousIntentBeforeObservations(t *testing.T) {
	service := Service{}
	for _, confirm := range []bool{false, true} {
		got, err := service.Reconcile(t.Context(), "state", model.ExecutionReconcileRequest{ID: "cycle", Preview: confirm, Confirm: confirm}, port.ExecutionReconcileDependencies{})
		if got.ID != "cycle" || got.OK || err == nil || err.Error() != "execution reconcile requires exactly one of preview or confirm" {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	}
}
