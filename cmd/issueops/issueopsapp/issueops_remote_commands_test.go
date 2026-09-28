package issueopsapp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	core "issueops/internal/adapter/issueops"
	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopsbodysync"
	"issueops/internal/port"
)

func TestBodySyncCommandCompositionValidatesBeforeProviderEffects(t *testing.T) {
	root := t.TempDir()
	record, err := core.StartIssueOps(root, model.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "69-body-command"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/69"
	if _, err = core.WriteIssueOps(root, record); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"provider-error", "empty-body", "secret-body", "ancestry-error", "preview"} {
		t.Run(mode, func(t *testing.T) {
			provider := &bodySyncWiringProvider{body: "old body", updated: true}
			resolved, observed := 0, 0
			service := newBodySyncCommand(root, func(name string) (port.IssueProvider, error) {
				resolved++
				if name != "github" {
					t.Fatalf("provider=%s", name)
				}
				if mode == "provider-error" {
					return nil, errors.New("provider unavailable")
				}
				return provider, nil
			}, func() ([]model.NativeProcessReceipt, error) {
				observed++
				if mode == "ancestry-error" {
					return nil, errors.New("ancestry unavailable")
				}
				return []model.NativeProcessReceipt{{PID: 1}}, nil
			})
			body := "new body"
			if mode == "empty-body" || mode == "provider-error" {
				body = " "
			}
			if mode == "secret-body" {
				body = "token=super-secret"
			}
			_, result, err := service.Sync(context.Background(), application.BodySyncInput{Command: contract.Command{ID: record.ID, Kind: contract.KindIssue, ProposedBody: body}})
			if resolved != 1 || provider.writes != 0 {
				t.Fatalf("resolved=%d writes=%d", resolved, provider.writes)
			}
			if mode == "preview" {
				if err != nil || result.Preview != "preview" || observed != 1 {
					t.Fatalf("result=%+v err=%v observed=%d", result, err, observed)
				}
			} else {
				want := map[string]string{"provider-error": "provider unavailable", "empty-body": "replacement body is required", "secret-body": "secret", "ancestry-error": "ancestry unavailable"}[mode]
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("err=%v want=%s", err, want)
				}
				if mode != "ancestry-error" && observed != 0 {
					t.Fatal("observed ancestry before input rejection")
				}
			}
			stored, err := core.ReadIssueOps(root, record.ID)
			if err != nil || len(stored.BodySyncs) != 0 {
				t.Fatalf("preview/failure persisted: %+v err=%v", stored.BodySyncs, err)
			}
		})
	}
}

func TestPublicationCommandCompositionPreservesDefaultsAndActor(t *testing.T) {
	root := t.TempDir()
	record, err := core.StartIssueOps(root, model.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "70-command"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/70"
	record.BranchPrepare = &model.IssueOpsBranchPrepare{Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch, BaseBranch: "release", LinkVerified: true}
	if _, err = core.WriteIssueOps(root, record); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(path, []byte("  publication body\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ancestry, err := core.ObserveNativeProcessAncestry(os.Getpid())
	if err != nil || len(ancestry) == 0 {
		t.Fatalf("ancestry=%v err=%v", ancestry, err)
	}
	observed, published := 0, 0
	service := newPublicationCommand(root, func(_ context.Context, _ string, req model.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
		published++
		if req.Provider != "github" || req.Head != record.Branch || req.Base != "release" || req.Body != "publication body" || strings.Join(req.Labels, ",") != "bug" || strings.Join(req.Assignees, ",") != "owner" {
			t.Fatalf("request=%+v", req)
		}
		if req.Confirm && (req.Actor.Host != "codex" || req.Actor.SessionID != "session" || len(req.Actor.ProcessAncestry) == 0) {
			t.Fatalf("actor=%+v", req.Actor)
		}
		return port.IssueProviderCreatePullRequestResult{OK: true, Preview: "preview"}, nil
	}, func() ([]model.NativeProcessReceipt, error) { observed++; return ancestry, nil })
	input := application.PublicationInput{Request: model.RemotePullRequestRequest{ID: record.ID, Title: "PR", Labels: []string{" bug ", "bug"}, Assignees: []string{" owner "}, Actor: model.NativeActor{Host: " CODEX ", SessionID: " session ", SessionProcess: &ancestry[0]}}, BodyFile: path}
	if _, err := service.Create(context.Background(), input); err != nil || observed != 0 || published != 1 {
		t.Fatalf("preview err=%v observed=%d published=%d", err, observed, published)
	}
	input.Request.Confirm = true
	if _, err := service.Create(context.Background(), input); err != nil || observed != 1 || published != 2 {
		t.Fatalf("confirm err=%v observed=%d published=%d", err, observed, published)
	}
	input.BodyFile = ""
	input.Request.Body = "token=super-secret"
	if _, err := service.Create(context.Background(), input); err == nil || observed != 1 || published != 2 {
		t.Fatalf("unsafe body err=%v observed=%d published=%d", err, observed, published)
	}
}
