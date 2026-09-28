package issueopsapp

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/issueops"
	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type issueReconcileProvider struct {
	port.IssueProvider
	result port.IssueProviderFindIssueCreateCandidatesResult
	calls  int
}

func (p *issueReconcileProvider) FindIssueCreateCandidates(_ context.Context, req port.IssueProviderFindIssueCreateCandidatesRequest) (port.IssueProviderFindIssueCreateCandidatesResult, error) {
	p.calls++
	if req.ProjectAuthority != "github.com/acme/repo" || req.Marker == "" {
		return p.result, fmt.Errorf("missing sealed search constraints")
	}
	return p.result, nil
}

func TestIssueReconcileCompositionPreservesPreviewAndRecoversVerificationFailure(t *testing.T) {
	root := t.TempDir()
	record, err := issueops.StartIssueOps(root, model.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "51-reconcile"})
	if err != nil {
		t.Fatal(err)
	}
	body := "sealed body"
	digest := sha256.Sum256([]byte(body))
	record, err = application.NewIssueCreateIntents(issueops.IssueCreateIntentStore{StateRoot: root}, time.Now).Begin(context.Background(), record.ID, model.IssueOpsIssueCreateIntentRequest{OperationID: strings.Repeat("1", 32), Provider: "github", ProjectAuthority: "github.com/acme/repo", Title: "Title", BodySHA256: fmt.Sprintf("%x", digest), StartedAt: "2026-09-28T00:00:00Z", Labels: []string{"bug"}, Assignees: []string{"owner"}})
	if err != nil {
		t.Fatal(err)
	}
	provider := &issueReconcileProvider{result: port.IssueProviderFindIssueCreateCandidatesResult{Candidates: []port.IssueProviderIssueCreateCandidate{{URL: "https://github.com/acme/repo/issues/51", Title: "Title", Body: body}}}}
	verified := 0
	verificationErr := errors.New("token=super-secret verification failed")
	now := func() time.Time { return time.Date(2026, 9, 28, 1, 0, 0, 0, time.UTC) }
	service := newIssueReconciler(root, func(name string) (port.IssueProvider, error) {
		if name != "github" {
			t.Fatalf("provider = %s", name)
		}
		return provider, nil
	}, func(_ context.Context, req model.IssueOpsRemoteArtifactVerificationRequest) error {
		verified++
		if req.URL != provider.result.Candidates[0].URL || strings.Join(req.Labels, ",") != "bug" || strings.Join(req.Assignees, ",") != "owner" {
			t.Fatalf("verify request = %+v", req)
		}
		return verificationErr
	}, now)
	preview, err := service.Reconcile(context.Background(), record.ID, false)
	if err != nil || !preview.WouldAdopt || preview.IssueURL != "" || verified != 0 {
		t.Fatalf("preview = %+v, %v; verified=%d", preview, err, verified)
	}
	stored, err := issueops.ReadIssueOps(root, record.ID)
	if err != nil || !reflect.DeepEqual(stored, record) {
		t.Fatalf("preview changed storage: %v", err)
	}
	if _, err := service.Reconcile(context.Background(), record.ID, true); !errors.Is(err, verificationErr) {
		t.Fatalf("verification = %v", err)
	}
	failed, err := issueops.ReadIssueOps(root, record.ID)
	if err != nil || failed.IssueURL != "" || failed.IssueCreateIntent.Status != model.IssueCreateIntentVerificationFailed || strings.Contains(failed.IssueCreateIntent.Failure, "super-secret") {
		t.Fatalf("failed receipt = %+v, %v", failed.IssueCreateIntent, err)
	}
	verificationErr = nil
	result, err := service.Reconcile(context.Background(), record.ID, true)
	if err != nil || result.WouldAdopt || result.IssueURL != provider.result.Candidates[0].URL || result.IssueCreateIntent.Status != model.IssueCreateIntentCompleted {
		t.Fatalf("adoption = %+v, %v", result, err)
	}
	if _, err := service.Reconcile(context.Background(), record.ID, true); err == nil || provider.calls != 3 {
		t.Fatalf("completed intent searched again: calls=%d err=%v", provider.calls, err)
	}
}
