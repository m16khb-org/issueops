package issueopsapp

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/issueops"
	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type issueCreateProvider struct {
	port.IssueProvider
	create func(context.Context, port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error)
}

func (p issueCreateProvider) CreateIssueContext(ctx context.Context, req port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	return p.create(ctx, req)
}

func TestIssueCreateCancellationPreservesPreviewAndConfirm(t *testing.T) {
	for _, confirm := range []bool{false, true} {
		name := "preview"
		if confirm {
			name = "confirm"
		}
		t.Run(name, func(t *testing.T) {
			root, repo := t.TempDir(), t.TempDir()
			for _, args := range [][]string{{"init", "-q", repo}, {"-C", repo, "remote", "add", "origin", "https://github.com/acme/repo.git"}} {
				if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
					t.Fatalf("git: %s %v", out, err)
				}
			}
			record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: repo, Branch: "74-cancel"})
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			provider := issueCreateProvider{create: func(ctx context.Context, req port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
				calls++
				if req.Confirm != confirm || (ctx.Err() != nil) != confirm {
					t.Fatalf("confirm=%v error=%v", req.Confirm, ctx.Err())
				}
				if confirm {
					return port.IssueProviderCreateIssueResult{}, &port.IssueProviderCreateError{Invoked: false, Err: ctx.Err()}
				}
				return port.IssueProviderCreateIssueResult{OK: true, Preview: "preview"}, nil
			}}
			service := newIssueCreator(root, func(string) (port.IssueProvider, error) { return provider, nil }, func(context.Context, model.IssueOpsRemoteArtifactVerificationRequest) error {
				t.Fatal("unexpected verification")
				return nil
			}, time.Now)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_, err = service.Create(ctx, application.IssueCreateCommand{ID: record.ID, Provider: "github", Title: "Title", Body: readableWiringIssueBody, Template: "implementation_task", Labels: []string{"bug"}, Assignees: []string{"owner"}, Confirm: confirm})
			if calls != 1 || (err != nil) != confirm {
				t.Fatalf("calls=%d error=%v", calls, err)
			}
			stored, err := issueops.ReadIssueOps(root, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !confirm && stored.IssueCreateIntent != nil {
				t.Fatal("preview persisted intent")
			}
			if confirm && (stored.IssueCreateIntent == nil || stored.IssueCreateIntent.Status != model.IssueCreateIntentNotInvoked) {
				t.Fatalf("intent=%+v", stored.IssueCreateIntent)
			}
		})
	}
}

func TestIssueCreateCompositionPersistsBeforeInvocationAndBlocksAmbiguousRetry(t *testing.T) {
	root, repo := t.TempDir(), t.TempDir()
	for _, args := range [][]string{{"init", "-q", repo}, {"-C", repo, "remote", "add", "origin", "https://github.com/acme/repo.git"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git: %s %v", out, err)
		}
	}
	record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: repo, Branch: "52-issue-create"})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	invoked := false
	cancel := func() {}
	provider := issueCreateProvider{create: func(_ context.Context, req port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
		calls++
		stored, err := issueops.ReadIssueOps(root, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !req.Confirm {
			if stored.IssueCreateIntent != nil || req.Body != readableWiringIssueBody {
				t.Fatal("preview sealed an intent")
			}
			return port.IssueProviderCreateIssueResult{OK: true, Preview: "preview"}, nil
		}
		if stored.IssueCreateIntent == nil || stored.IssueCreateIntent.Status != model.IssueCreateIntentPending || !strings.Contains(req.Body, stored.IssueCreateIntent.Marker) || req.ProjectKey != "github.com/acme/repo" {
			t.Fatalf("provider called before durable seal: %+v", stored.IssueCreateIntent)
		}
		if invoked {
			cancel()
		}
		return port.IssueProviderCreateIssueResult{}, &port.IssueProviderCreateError{Invoked: invoked, Err: errors.New("connection lost")}
	}}
	service := newIssueCreator(root, func(name string) (port.IssueProvider, error) {
		if name != "github" {
			t.Fatalf("provider=%s", name)
		}
		return provider, nil
	}, func(context.Context, model.IssueOpsRemoteArtifactVerificationRequest) error {
		t.Fatal("failed creation verified")
		return nil
	}, time.Now)
	cmd := application.IssueCreateCommand{ID: record.ID, Title: "Title", Body: readableWiringIssueBody, Template: "implementation_task", Labels: []string{"bug"}, Assignees: []string{"owner"}}
	if _, err := service.Create(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	cmd.Confirm = true
	if _, err := service.Create(context.Background(), cmd); err == nil {
		t.Fatal("expected provider failure")
	}
	first, err := issueops.ReadIssueOps(root, record.ID)
	if err != nil || first.IssueCreateIntent.Status != model.IssueCreateIntentNotInvoked {
		t.Fatalf("first failure=%+v %v", first.IssueCreateIntent, err)
	}
	invoked = true
	var ctx context.Context
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	if _, err := service.Create(ctx, cmd); err == nil {
		t.Fatal("expected ambiguous failure")
	}
	second, err := issueops.ReadIssueOps(root, record.ID)
	if err != nil || second.IssueCreateIntent.Status != model.IssueCreateIntentInvokedUnknown || second.IssueCreateIntent.OperationID != first.IssueCreateIntent.OperationID || second.IssueCreateIntent.Attempt != 2 {
		t.Fatalf("retry=%+v %v", second.IssueCreateIntent, err)
	}
	if _, err := service.Create(context.Background(), cmd); err == nil || !strings.Contains(err.Error(), "reconcile before retry") || calls != 3 {
		t.Fatalf("ambiguous retry: calls=%d err=%v", calls, err)
	}
}

func TestIssueCreateCompositionLinksReadyCycleAfterLiveVerification(t *testing.T) {
	root, repo := t.TempDir(), t.TempDir()
	for _, args := range [][]string{{"init", "-q", repo}, {"-C", repo, "remote", "add", "origin", "https://github.com/acme/repo.git"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git: %s %v", out, err)
		}
	}
	record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: repo, Branch: "53-create-ready"})
	if err != nil {
		t.Fatal(err)
	}
	record.Intent = &model.IssueOpsIntentContract{IntentClass: "trivial", RawRequest: "create issue", InterpretedIntent: "track work", SuccessCriteria: []string{"issue linked"}}
	if _, err := (issueops.CycleRecordStore{StateRoot: root}).Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	provider := issueCreateProvider{create: func(context.Context, port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
		return port.IssueProviderCreateIssueResult{OK: true, URL: "https://github.com/acme/repo/issues/53"}, nil
	}}
	verified := false
	service := newIssueCreator(root, func(string) (port.IssueProvider, error) { return provider, nil }, func(_ context.Context, req model.IssueOpsRemoteArtifactVerificationRequest) error {
		stored, err := issueops.ReadIssueOps(root, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if stored.IssueURL != "" || stored.IssueCreateIntent.Status != model.IssueCreateIntentPending {
			t.Fatal("linked before verification")
		}
		if req.Kind != "issue" || req.Provider != "github" || req.URL != "https://github.com/acme/repo/issues/53" || strings.Join(req.Labels, ",") != "bug" || strings.Join(req.Assignees, ",") != "owner" {
			t.Fatalf("verify=%+v", req)
		}
		verified = true
		return nil
	}, time.Now)
	result, err := service.Create(context.Background(), application.IssueCreateCommand{ID: record.ID, Title: "Title", Body: readableWiringIssueBody, Template: "implementation_task", Labels: []string{" bug ", "bug"}, Assignees: []string{"owner"}, Confirm: true})
	if err != nil || !verified || result.Provider != "github" || strings.Join(result.Labels, ",") != "bug" {
		t.Fatalf("create=%+v verified=%t err=%v", result, verified, err)
	}
	stored, err := issueops.ReadIssueOps(root, record.ID)
	if err != nil || stored.IssueURL != result.URL || stored.IssueCreateIntent.Status != model.IssueCreateIntentCompleted || stored.Phase != model.IssueOpsPhasePlan {
		t.Fatalf("completion phase=%s intent=%+v err=%v", stored.Phase, stored.IssueCreateIntent, err)
	}
}

const readableWiringIssueBody = `## 요약

이슈 본문을 요약이 먼저 오는 짧은 계약으로 바꿉니다. 끝나면 팀원이 요약만 읽고 변경 이유를 압니다.

## 배경

지금 본문은 절이 많아 팀원이 무엇이 바뀌는지 찾기 어렵습니다.

## 완료 기준

- 새로 게시한 이슈의 첫 절이 요약입니다.

## 범위

- 하는 것: 본문 계약 변경
- 하지 않는 것: 이미 게시된 이슈 수정

## 검증

단위 테스트로 필수 절과 요약 규칙을 확인합니다.`
