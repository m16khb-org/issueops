package feedbackcleanup

import (
	"context"
	"testing"

	issueopscore "issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	issuedomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

func wireAbandonCapture(t *testing.T) (*[]issueopscontract.CleanupAbandonRequest, *int, Command) {
	command := testCleanupCommand()
	t.Helper()
	requests := &[]issueopscontract.CleanupAbandonRequest{}
	providerCalls := new(int)
	wired := command.Operations
	wired.IssueOpsStateRoot = issueopscore.IssueOpsStateRoot
	wired.ReadIssueOps = issueopscore.ReadIssueOps
	wired.ResolveRecordProvider = issuedomain.ResolveRecordProvider
	wired.CleanupAbandon = func(_ context.Context, _ string, req issueopscontract.CleanupAbandonRequest, _ Deps) (issueopscontract.CleanupAbandonResult, error) {
		*requests = append(*requests, req)
		return issueopscontract.CleanupAbandonResult{OK: true, ID: req.ID, RemoteEffects: []string{"close_issue"}}, nil
	}
	command.Operations = wired
	_ = providerCalls
	return requests, providerCalls, command
}

// 세 플래그가 요청으로 그대로 전달돼야 어댑터의 게이트가 의미를 갖는다.
func TestRunCleanupAbandonForwardsRemoteEffectFlags(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record := cleanupStatusRecord(t, false, true)
	requests, providerCalls, command := wireAbandonCapture(t)
	deps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintJSON:  func(any) error { return nil },
		PrintError: func(error) error { return nil },
		Provider: func(name string) (port.IssueProvider, error) {
			*providerCalls++
			return &fakeCleanupAbandonProvider{}, nil
		},
	}
	err := command.RunCleanup([]string{
		"abandon", "--id", record.ID, "--reason", "폐기 검증",
		"--close-pr", "--close-issue", "--delete-remote-branch", "--preview", "--json",
	}, deps)
	if err != nil {
		t.Fatalf("abandon preview: %v", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("expected one abandon request, got %d", len(*requests))
	}
	got := (*requests)[0]
	if !got.ClosePR || !got.CloseIssue || !got.DeleteRemoteBranch {
		t.Fatalf("remote effect flags did not reach the adapter: %#v", got)
	}
	if *providerCalls != 0 {
		t.Fatalf("transport must not resolve the provider before executor ownership, got %d", *providerCalls)
	}
}

// 플래그가 없으면 provider를 해석하지 않는다. 원격 정체가 없는 사이클도
// 폐기할 수 있어야 한다.
func TestRunCleanupAbandonWithoutFlagsNeedsNoProvider(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record := feedbackCleanupIssueOpsRecord(t)
	requests, providerCalls, command := wireAbandonCapture(t)
	deps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintJSON:  func(any) error { return nil },
		PrintError: func(error) error { return nil },
		Provider: func(string) (port.IssueProvider, error) {
			*providerCalls++
			return nil, context.DeadlineExceeded
		},
	}
	if err := command.RunCleanup([]string{"abandon", "--id", record.ID, "--reason", "플래그 없는 폐기", "--preview", "--json"}, deps); err != nil {
		t.Fatalf("abandon preview: %v", err)
	}
	got := (*requests)[0]
	if got.ClosePR || got.CloseIssue || got.DeleteRemoteBranch {
		t.Fatalf("no flag must mean no remote effect: %#v", got)
	}
	if *providerCalls != 0 {
		t.Fatalf("no flag must resolve no provider, got %d", *providerCalls)
	}
}

type fakeCleanupAbandonProvider struct{}

func (fakeCleanupAbandonProvider) Name() string { return "github" }
func (fakeCleanupAbandonProvider) CreateIssue(port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	return port.IssueProviderCreateIssueResult{}, nil
}
func (fakeCleanupAbandonProvider) CreatePullRequest(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	return port.IssueProviderCreatePullRequestResult{}, nil
}
func (fakeCleanupAbandonProvider) CreateChild(port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	return port.IssueProviderCreateChildResult{}, nil
}
func (fakeCleanupAbandonProvider) CloseChild(port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	return port.IssueProviderCloseChildResult{}, nil
}
func (fakeCleanupAbandonProvider) CloseIssue(context.Context, port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error) {
	return port.IssueProviderCloseIssueResult{}, nil
}
func (fakeCleanupAbandonProvider) UpdateIssueBodySection(context.Context, port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	return port.IssueProviderUpdateIssueBodySectionResult{}, nil
}
