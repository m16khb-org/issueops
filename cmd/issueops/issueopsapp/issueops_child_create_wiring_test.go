package issueopsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/testsupport"
)

// TestChildCreateCLIRecordsFailureCause runs the production CLI against a fake
// gh whose child issue create fails with HTTP 404 and checks that status
// reports the failing command and HTTP status on the child operation.
func TestChildCreateCLIRecordsFailureCause(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root := issueOpsStateRoot()
	record, err := startIssueOpsFixture(root, model.StartRequest{Repo: makeGitRepoForContract(t), Branch: "1234-child-cause"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = newIssueLinker(root).Issue(context.Background(), record.ID, "https://github.com/acme/repo/issues/1234", nil)
	if err != nil {
		t.Fatal(err)
	}
	record, err = newBranchPreparer(root).Prepare(context.Background(), record.ID, model.BranchPrepareRequest{Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch, BaseBranch: "main", LinkVerified: true}, nil)
	if err != nil {
		t.Fatal(err)
	}

	bin, unexpected := t.TempDir(), filepath.Join(t.TempDir(), "unexpected-gh-call")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1 $2 $3" = "issue create --help" ]; then printf "  --title string\n";exit 0;fi
if [ "$1 $2" = "issue create" ]; then
 printf 'HTTP 404: Not Found\n' >&2;exit 1
fi
printf '%%s\n' "$*" >>%q
printf 'unexpected gh call: %%s\n' "$*" >&2
exit 97
`, filepath.ToSlash(unexpected))
	testsupport.WriteFakeCommand(t, bin, "gh", script)
	t.Setenv("PATH", bin)
	deps := issueOpsCLIDependencies()

	out, err := runCLIForError(t, deps, "remote", "create-child", "--id", record.ID, "--title", "Child", "--body", readableWiringChildBody, "--label", "bug", "--assignee", "octocat", "--confirm", "--json")
	if err == nil {
		t.Fatalf("create-child succeeded against a failing gh: %s", out)
	}
	var operations []json.RawMessage
	if err := json.Unmarshal(statusFieldForCause(t, deps, record.ID, "child_create_operations"), &operations); err != nil {
		t.Fatal(err)
	}
	if len(operations) != 1 {
		t.Fatalf("child operations: %d", len(operations))
	}
	assertFailureCause(t, operations[0])
	if data, readErr := os.ReadFile(unexpected); readErr == nil {
		t.Fatalf("fake gh saw unexpected calls:\n%s", data)
	} else if !errors.Is(readErr, os.ErrNotExist) {
		t.Fatal(readErr)
	}
}
