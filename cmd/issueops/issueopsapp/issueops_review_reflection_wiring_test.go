package issueopsapp

import (
	"context"
	"strings"
	"testing"
	"time"

	core "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestReviewReflectionCompositionRechecksHolderBeforeStamp(t *testing.T) {
	root, repo, worktree := t.TempDir(), t.TempDir(), t.TempDir()
	record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: repo, Branch: "63-review-reflection"})
	if err != nil {
		t.Fatal(err)
	}
	process := liveFixtureReceipt(t)
	record.IssueURL = "https://github.com/acme/repo/issues/63"
	record.DevilsAdvocateReview = &model.IssueOpsDevilsAdvocateReview{Verdict: "stop", Findings: []string{"review finding"}, RecordedAt: "then"}
	record.Execution = &model.Execution{Mode: model.ExecutionModeDirect, Workspace: model.Workspace{SourceRoot: repo, Root: worktree, Branch: record.Branch, BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "then"}, Lease: model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &model.NativeActor{Host: "codex", SessionID: "holder", SessionProcess: &process}, ClaimedAt: "then"}}
	if _, err := (core.CycleRecordStore{StateRoot: root}).Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	actor := model.IssueOpsActor{Host: "codex", SessionID: "holder", CWD: worktree}
	provider := &completionProvider{}
	mode := "preview"
	provider.update = func(req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
		if req.Section != model.IssueBodySectionDevilsAdvocate || req.IssueURL != record.IssueURL || strings.Join(req.Findings, ",") != "review finding" {
			t.Fatalf("request=%+v", req)
		}
		if mode == "transfer" || mode == "disappear" || mode == "apply" {
			latest, e := core.ReadIssueOps(root, record.ID)
			if e != nil {
				t.Fatal(e)
			}
			latest.AISlopCleanVerification = []string{"concurrent evidence"}
			if mode == "transfer" {
				latest.Execution.Lease.Holder.SessionID = "new-holder"
			}
			if mode == "disappear" {
				latest.DevilsAdvocateReview = nil
			}
			if _, e := (core.CycleRecordStore{StateRoot: root}).Save(context.Background(), latest); e != nil {
				t.Fatal(e)
			}
		}
		return port.IssueProviderUpdateIssueBodySectionResult{OK: true, Updated: mode != "preview", Preview: "preview", URL: req.IssueURL}, nil
	}
	service := newReviewReflectionService(root, func(string) (port.IssueProvider, error) { return provider, nil }, func() ([]model.NativeProcessReceipt, error) { return []model.NativeProcessReceipt{process}, nil }, func() time.Time { return time.Date(2026, 9, 28, 1, 0, 0, 0, time.UTC) })
	foreign := actor
	foreign.SessionID = "foreign"
	if _, _, err := service.Reflect(context.Background(), record.ID, "", true, foreign); err == nil || provider.updates != 0 {
		t.Fatalf("foreign holder err=%v updates=%d", err, provider.updates)
	}
	outside := actor
	outside.CWD = repo
	if _, _, err := service.Reflect(context.Background(), record.ID, "", true, outside); err == nil || !strings.Contains(err.Error(), "canonical worktree cwd") || provider.updates != 0 {
		t.Fatalf("foreign cwd err=%v updates=%d", err, provider.updates)
	}
	got, _, err := service.Reflect(context.Background(), record.ID, "", false, actor)
	if err != nil || got.DevilsAdvocateReview.IssueReflectedAt != "" {
		t.Fatalf("preview=%+v err=%v", got, err)
	}
	got, _, err = service.Reflect(context.Background(), record.ID, "", true, actor)
	if err != nil || got.DevilsAdvocateReview.IssueReflectedAt != "" {
		t.Fatalf("unapplied update stamped: %v", err)
	}
	for _, tc := range []struct{ mode, want string }{{"transfer", "current write lease holder"}, {"disappear", "review disappeared"}, {"apply", ""}} {
		mode = tc.mode
		if _, err := (core.CycleRecordStore{StateRoot: root}).Save(context.Background(), record); err != nil {
			t.Fatal(err)
		}
		got, _, err = service.Reflect(context.Background(), record.ID, "", true, actor)
		latest, readErr := core.ReadIssueOps(root, record.ID)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if tc.want != "" {
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s: err=%v", mode, err)
			}
			if latest.DevilsAdvocateReview != nil && latest.DevilsAdvocateReview.IssueReflectedAt != "" {
				t.Fatal("failed authorization stamped")
			}
		} else if err != nil || got.DevilsAdvocateReview.IssueReflectedAt != "2026-09-28T01:00:00Z" || strings.Join(got.AISlopCleanVerification, ",") != "concurrent evidence" {
			t.Fatalf("reflection lost latest evidence: %+v err=%v", got, err)
		}
	}
}
