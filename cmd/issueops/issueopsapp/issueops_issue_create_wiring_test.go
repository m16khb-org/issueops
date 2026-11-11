package issueopsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/cmd/issueops/issueopscli"
	"issueops/internal/adapter/issueops"
	application "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
	"issueops/internal/testsupport"
)

type issueCreateProvider struct {
	port.IssueProvider
	create func(context.Context, port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error)
}

func (p issueCreateProvider) CreateIssue(ctx context.Context, req port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
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
			record, err := startIssueOpsFixture(root, model.StartRequest{Repo: repo, Branch: "74-cancel"})
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
			service := newIssueCreator(root, func(string) (port.IssueProvider, error) { return provider, nil }, func(context.Context, model.RemoteArtifactVerificationRequest) error {
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
	record, err := startIssueOpsFixture(root, model.StartRequest{Repo: repo, Branch: "52-issue-create"})
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
	}, func(context.Context, model.RemoteArtifactVerificationRequest) error {
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

// runCLIForError runs one production CLI command that is expected to fail and
// returns its stdout and error; captureStdoutForContract fails the test on a
// command error, so a failed provider call needs its own capture.
func runCLIForError(t *testing.T, deps issueopscli.Dependencies, args ...string) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan []byte, 1)
	go func() {
		out, _ := io.ReadAll(r)
		done <- out
	}()
	os.Stdout = w
	runErr := issueopscli.RunIssueOpsWithDependencies(args, deps)
	closeErr := w.Close()
	os.Stdout = oldStdout
	out := <-done
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return string(out), runErr
}

// statusFieldForCause reads one top-level record field through `status --field`
// and returns its raw JSON value. The output shape is
// {"ok":..,"id":..,"phase":..,"<field>":<value>}.
func statusFieldForCause(t *testing.T, deps issueopscli.Dependencies, id, field string) json.RawMessage {
	t.Helper()
	raw := captureStdoutForContract(t, func() error {
		return issueopscli.RunIssueOpsWithDependencies([]string{"status", "--id", id, "--field", field, "--json"}, deps)
	})
	var projection map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &projection); err != nil {
		t.Fatalf("status output is not JSON: %v\n%s", err, raw)
	}
	value, ok := projection[field]
	if !ok {
		t.Fatalf("status output has no %q: %s", field, raw)
	}
	return value
}

// assertFailureCause checks the machine-readable failure cause of an
// invoked_unknown intent or child operation.
func assertFailureCause(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	if got["status"] != "invoked_unknown" || got["failure_command"] != "gh issue create" || got["failure_http_status"] != float64(404) {
		t.Fatalf("failure cause: %s", raw)
	}
	if failure, _ := got["failure"].(string); failure == "" {
		t.Fatalf("failure text is empty: %s", raw)
	}
}

// TestIssueCreateCLIRecordsFailureCause runs the production CLI against a fake
// gh whose issue create fails with HTTP 404 and checks that status reports the
// failing command and HTTP status next to the failure text.
func TestIssueCreateCLIRecordsFailureCause(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := makeGitRepoForContract(t)
	runGitForContract(t, repo, "remote", "add", "origin", "https://github.com/acme/repo.git")
	record, err := startIssueOpsFixture(issueOpsStateRoot(), model.StartRequest{Repo: repo, Branch: "52-issue-cause"})
	if err != nil {
		t.Fatal(err)
	}
	bin, unexpected := t.TempDir(), filepath.Join(t.TempDir(), "unexpected-gh-call")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1 $2" = "issue create" ]; then
 printf 'HTTP 404: Not Found\n' >&2;exit 1
fi
printf '%%s\n' "$*" >>%q
printf 'unexpected gh call: %%s\n' "$*" >&2
exit 97
`, filepath.ToSlash(unexpected))
	testsupport.WriteFakeCommand(t, bin, "gh", script)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+gitOnlyPath(t))
	deps := issueOpsCLIDependencies()

	out, err := runCLIForError(t, deps, "remote", "create-issue", "--id", record.ID, "--provider", "github", "--title", "Title", "--body", readableWiringIssueBody, "--template", "implementation_task", "--label", "bug", "--assignee", "owner", "--confirm", "--json")
	if err == nil {
		t.Fatalf("create-issue succeeded against a failing gh: %s", out)
	}
	assertFailureCause(t, statusFieldForCause(t, deps, record.ID, "issue_create_intent"))
	if data, readErr := os.ReadFile(unexpected); readErr == nil {
		t.Fatalf("fake gh saw unexpected calls:\n%s", data)
	} else if !errors.Is(readErr, os.ErrNotExist) {
		t.Fatal(readErr)
	}
}

func TestIssueCreateCompositionLinksReadyCycleAfterLiveVerification(t *testing.T) {
	root, repo := t.TempDir(), t.TempDir()
	for _, args := range [][]string{{"init", "-q", repo}, {"-C", repo, "remote", "add", "origin", "https://github.com/acme/repo.git"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git: %s %v", out, err)
		}
	}
	record, err := startIssueOpsFixture(root, model.StartRequest{Repo: repo, Branch: "53-create-ready"})
	if err != nil {
		t.Fatal(err)
	}
	record.Intent = &model.IntentContract{IntentClass: "trivial", RawRequest: "create issue", InterpretedIntent: "track work", SuccessCriteria: []string{"issue linked"}}
	if _, err := (issueops.CycleRecordStore{StateRoot: root}).Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	provider := issueCreateProvider{create: func(context.Context, port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
		return port.IssueProviderCreateIssueResult{OK: true, URL: "https://github.com/acme/repo/issues/53"}, nil
	}}
	verified := false
	service := newIssueCreator(root, func(string) (port.IssueProvider, error) { return provider, nil }, func(_ context.Context, req model.RemoteArtifactVerificationRequest) error {
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
	if err != nil || stored.IssueURL != result.URL || stored.IssueCreateIntent.Status != model.IssueCreateIntentCompleted || stored.Phase != model.PhasePlan {
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
