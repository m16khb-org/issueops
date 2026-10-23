package issueopsapp

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	shelltoken "issueops/internal/domain/shelltoken"

	"issueops/cmd/issueops/issueopscli/executioncmd"
	issueopscore "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/preflight"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestIssueOpsPrepareWiringRunsRealDirectPreviewWithoutPersistence(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()
	claimWiringGit(t, repo, "init", "-q", "-b", "main")
	claimWiringGit(t, repo, "config", "user.name", "IssueOps Test")
	claimWiringGit(t, repo, "config", "user.email", "issueops@example.invalid")
	claimWiringGit(t, repo, "commit", "--allow-empty", "-q", "-m", "initial")
	baseHead := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "HEAD"))
	record, err := startIssueOpsFixture(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "199-preparation-wiring"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/199"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch,
		BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	if _, err := (issueopscore.CycleRecordStore{StateRoot: stateRoot}).Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	direct := &preparationWiringDirectFake{}
	handler := newIssueOpsPreparationHandler(issueOpsPreparationCompositionDeps{
		Direct: direct, Now: func() time.Time { return time.Date(2026, 8, 2, 4, 5, 6, 7, time.UTC) },
	})
	process := &issueopscontract.NativeProcessReceipt{PID: 199, StartedAt: "2026-08-02T00:00:00Z", Executable: "/usr/local/bin/codex"}

	result, err := handler(context.Background(), stateRoot, issueopscontract.ExecutionPrepareRequest{
		ID: record.ID, Actor: issueopscontract.NativeActor{Host: "codex", SessionID: "session", SessionProcess: process},
		CWD: repo, Confirm: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Preview || direct.calls != 1 {
		t.Fatalf("result=%#v direct calls=%d", result, direct.calls)
	}
	persisted, err := issueopscore.ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution != nil || persisted.WorktreePath != "" {
		t.Fatalf("preview persisted execution: %#v", persisted)
	}
}

func TestIssueOpsPrepareWiringPreviewNextCommandConfirmsRealPreparation(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateRoot)
	repo := t.TempDir()
	claimWiringGit(t, repo, "init", "-q", "-b", "main")
	claimWiringGit(t, repo, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "--allow-empty", "-q", "-m", "initial")
	baseHead := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "HEAD"))
	record, err := startIssueOpsFixture(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "199-preparation-confirm"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/199"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch,
		BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	if _, err := (issueopscore.CycleRecordStore{StateRoot: stateRoot}).Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	handler := productionIssueOpsExecutionDependencies().Prepare
	actor := claimWiringActor(t)
	previewRaw, err := newExecutionService().Execute(context.Background(), stateRoot, issueopscontract.ExecutionActionRequest{
		Action: issueopscontract.ExecutionActionPrepare, ID: record.ID, Actor: actor, CWD: repo,
	}, port.ExecutionActionDependencies{Prepare: handler})
	if err != nil {
		t.Fatal(err)
	}
	preview := previewRaw.(issueopscontract.ExecutionPrepareResult)
	if !preview.OK || !preview.Preview || preview.Execution != nil {
		t.Fatalf("preview=%#v", preview)
	}
	tokens := shelltoken.SplitCommandTokens(preview.NextCommand)
	if len(tokens) < 4 || strings.Join(tokens[:3], " ") != "issueops execution prepare" {
		t.Fatalf("invalid next command: %q tokens=%v", preview.NextCommand, tokens)
	}
	var raw any
	err = executioncmd.Run(tokens[2:], executioncmd.Deps{
		Runtime:   newIssueOpsExecutionRunners(),
		StateRoot: func() string { return stateRoot }, Prepare: handler,
		PrintJSON: func(value any) error { raw = value; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := raw.(issueopscontract.ExecutionPrepareResult)
	if !ok || !result.OK || result.Preview || result.Execution == nil || result.Execution.Lease.Generation != 1 {
		t.Fatalf("result=%#v", raw)
	}
}

func TestIssueOpsPrepareWiringRejectsActorBeforeStateMutation(t *testing.T) {
	stateRoot := t.TempDir()
	called := false
	handler := newIssueOpsPreparationHandler(issueOpsPreparationCompositionDeps{
		ValidateActor: func(context.Context, issueopscontract.NativeActor) error {
			called = true
			return errors.New("native session process receipt is not in the local process ancestry")
		},
	})

	result, err := handler(
		context.Background(),
		stateRoot,
		issueopscontract.ExecutionPrepareRequest{ID: "io-forged-actor"},
	)

	if err == nil || !strings.Contains(err.Error(), "not in the local process ancestry") {
		t.Fatalf("prepare error = %v", err)
	}
	if !called || result.ID != "io-forged-actor" {
		t.Fatalf("validator called=%v result=%+v", called, result)
	}
	entries, readErr := os.ReadDir(stateRoot)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("state root mutated before actor validation: %v", entries)
	}
}

type preparationWiringDirectFake struct{ calls int }

func (fake *preparationWiringDirectFake) Prepare(_ context.Context, request port.ExecutionWorkspaceRequest) (port.ExecutionWorkspaceReceipt, error) {
	fake.calls++
	return port.ExecutionWorkspaceReceipt{
		SourceRoot: request.SourceRoot, Root: request.Root, Branch: request.Branch,
		BaseHead: request.BaseHead, ParentWorktree: request.ParentWorktree}, nil
}
