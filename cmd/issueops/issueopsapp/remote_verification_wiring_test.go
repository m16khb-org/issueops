package issueopsapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/cmd/issueops/issueopscli"
	adapter "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
)

func TestRemoteVerificationWiringProtectsChildLink(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root := issueOpsStateRoot()
	record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: makeGitRepoForContract(t), Branch: "1234-child-verification"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = newIssueLinker(root).Issue(context.Background(), record.ID, "https://github.com/acme/repo/issues/1234", nil)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	gh := filepath.Join(bin, "gh")
	if err := os.WriteFile(gh, []byte("#!/bin/sh\necho 'HTTP 404: not found' >&2\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	deps := issueOpsCLIDependencies()
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	args := []string{"link-child", "--id", record.ID, "--child-url", "https://github.com/acme/repo/issues/34", "--title", "Child", "--json"}
	var refused error
	captureStdoutForContract(t, func() error { refused = issueopscli.RunIssueOpsWithDependencies(args, deps); return nil })
	if refused == nil || !strings.Contains(refused.Error(), "verify GitHub child issue through gh failed") {
		t.Fatalf("refused=%v", refused)
	}
	stored, err := adapter.ReadIssueOps(root, record.ID)
	if err != nil || len(stored.IssueLinks) != 0 {
		t.Fatalf("link written after failed verification: %+v err=%v", stored.IssueLinks, err)
	}
	if err := os.WriteFile(gh, []byte("#!/bin/sh\n[ \"$*\" = 'issue view https://github.com/acme/repo/issues/34 --json url,state,title' ] || exit 2\nprintf '{}\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	captureStdoutForContract(t, func() error { return issueopscli.RunIssueOpsWithDependencies(args, deps) })
	stored, err = adapter.ReadIssueOps(root, record.ID)
	if err != nil || len(stored.IssueLinks) != 1 || stored.IssueLinks[0].URL != "https://github.com/acme/repo/issues/34" {
		t.Fatalf("verified link=%+v err=%v", stored.IssueLinks, err)
	}
}
