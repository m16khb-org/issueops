package issueopscli

import (
	"context"
	"errors"
	"issueops/cmd/issueops/issueopscli/remotecmd"
	executionissue "issueops/internal/contract/executionissue"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestExportedIssueOpsFacades(t *testing.T) {
	if err := runIssueOps([]string{"unknown"}); err == nil {
		t.Fatal("unknown issueops subcommand should fail")
	}
	if err := testRemoteVerifier().Verify(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{Provider: "github", Kind: "pr", URL: "not-a-url"}); err == nil {
		t.Fatal("invalid remote artifact URL should fail before provider inspection")
	}

	sentinel := errors.New("sentinel")
	cli := testIssueOpsCommand()
	cli.VerifyChild = func(string) error { return sentinel }
	_, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return cli.runIssueOpsLinkChild([]string{"--child-url", "https://github.com/acme/repo/issues/1", "--json"})
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("stubbed child verifier err=%v", err)
	}

}

func TestIssueOpsPublicationCreateRequiresComposedDependencies(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record, err := startIssueOpsFixture(issueOpsStateRootForTest(), issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "195-publication-wrapper"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsIssueForTest(issueOpsStateRootForTest(), record.ID, "https://github.com/acme/repo/issues/195")
	if err != nil {
		t.Fatal(err)
	}
	args := []string{
		"remote", "create-pr", "--id", record.ID, "--provider", "github", "--title", "PR", "--body", "Body",
		"--head", record.Branch, "--base", "main", "--label", "bug", "--assignee", "maintainer",
	}
	if err := runIssueOps(args); !errors.Is(err, issueopscontract.ErrRemotePullRequestCreateHandlerUnavailable) {
		t.Fatalf("zero dependency wrapper err=%v", err)
	}
	handlerCalls := 0
	err = runIssueOpsForTest(args, Dependencies{Remote: testRemoteCommand(), Publication: remotecmd.PublicationHandlers{Create: func(_ context.Context, _ string, request issueopscontract.RemotePullRequestRequest) (executionissue.IssueProviderCreatePullRequestResult, error) {
		handlerCalls++
		if request.ID != record.ID || request.Confirm {
			t.Fatalf("request=%#v", request)
		}
		return executionissue.IssueProviderCreatePullRequestResult{OK: true, Preview: "would create pull request"}, nil
	}}})
	if err != nil || handlerCalls != 1 {
		t.Fatalf("handlerCalls=%d err=%v", handlerCalls, err)
	}
}

func TestIssueOpsDecisionAndCleanupCLIBranches(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record, err := startIssueOpsFixture(issueOpsStateRootForTest(), issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "123-decision"})
	if err != nil {
		t.Fatal(err)
	}
	if err := testIssueOpsCommand().runIssueOpsDecision(nil); err != nil {
		t.Fatalf("decision help: %v", err)
	}
	if err := testIssueOpsCommand().runIssueOpsDecision([]string{"remove"}); err == nil {
		t.Fatal("unknown decision subcommand should fail")
	}
	if err := testIssueOpsCommand().runIssueOpsDecision([]string{
		"add",
		"--id", record.ID,
		"--title", "Use focused tests",
		"--body", "Raise low coverage with boundary tests",
		"--kind", "test",
		"--rationale", "quality gate",
		"--alternative", "change threshold",
		"--affected-artifact", "test",
		"--json",
	}); err != nil {
		t.Fatalf("decision add: %v", err)
	}
}

func TestIssueOpsSubcommandSuggestions(t *testing.T) {
	cases := []struct {
		input        string
		mustContain  string
		mustNotMatch bool
	}{
		// concept hint: subcommand으로 오인되는 도메인 어휘.
		{"grill", "issueops phase --to grill", false},
		{"split", "issueops remote create-child", false},
		{"problem", "issueops phase --to problem", false},
		{"implement", "issueops phase --to implement", false},
		// 실제 registry에 대한 prefix 일치.
		{"domain", "domain-review", false},
		{"compat", "compatibility", false},
		{"execut", "execution", false},
		// 무의미한 입력에는 제안 없이 에러만 낸다.
		{"totally-bogus", "", true},
	}
	for _, tc := range cases {
		err := runIssueOps([]string{tc.input})
		if err == nil {
			t.Fatalf("input %q should fail", tc.input)
		}
		msg := err.Error()
		if !strings.Contains(msg, `unknown issueops subcommand`) {
			t.Fatalf("input %q: error missing canonical prefix: %q", tc.input, msg)
		}
		if tc.mustNotMatch {
			if strings.Contains(msg, "did you mean") {
				t.Fatalf("input %q: should have no suggestion, got %q", tc.input, msg)
			}
			continue
		}
		if !strings.Contains(msg, "did you mean") || !strings.Contains(msg, tc.mustContain) {
			t.Fatalf("input %q: expected suggestion containing %q, got %q", tc.input, tc.mustContain, msg)
		}
	}
}
