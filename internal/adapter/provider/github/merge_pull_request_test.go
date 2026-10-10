package github

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

const ghMergeTestURL = "https://github.com/acme/repo/pull/12"

// A legacy failing status context outranks a running check run, and the
// provider's BLOCKED is not double-reported when checks already explain it.
func TestGitHubMergeStateFoldsCheckRollup(t *testing.T) {
	state, err := parseGhMergeState([]byte(`{"url":"` + ghMergeTestURL + `","state":"OPEN","isDraft":true,"headRefOid":"abc","baseRefName":"main","mergeable":"CONFLICTING","mergeStateStatus":"BLOCKED","statusCheckRollup":[
		{"__typename":"CheckRun","name":"verify","status":"COMPLETED","conclusion":"SUCCESS"},
		{"__typename":"CheckRun","name":"lint","status":"IN_PROGRESS","conclusion":""},
		{"__typename":"CheckRun","name":"docs","status":"COMPLETED","conclusion":"SKIPPED"},
		{"__typename":"StatusContext","context":"ci/legacy","state":"FAILURE"}],"mergeCommit":null}`))
	if err != nil {
		t.Fatal(err)
	}
	if state.State != "open" || !state.Draft || state.Checks != model.RemoteChecksFailing || !state.Conflict || state.Blocked != "" {
		t.Fatalf("state = %+v", state)
	}
	if !reflect.DeepEqual(state.Failing, []string{"ci/legacy"}) || !reflect.DeepEqual(state.Pending, []string{"lint"}) {
		t.Fatalf("failing=%v pending=%v", state.Failing, state.Pending)
	}
	clean, err := parseGhMergeState([]byte(`{"state":"OPEN","mergeable":"MERGEABLE","mergeStateStatus":"BLOCKED","statusCheckRollup":[{"__typename":"CheckRun","name":"verify","status":"COMPLETED","conclusion":"SUCCESS"}]}`))
	if err != nil || clean.Checks != model.RemoteChecksPassing || clean.Blocked == "" {
		t.Fatalf("passing checks with BLOCKED must surface the block: %+v err=%v", clean, err)
	}
	none, err := parseGhMergeState([]byte(`{"state":"OPEN","statusCheckRollup":[]}`))
	if err != nil || none.Checks != model.RemoteChecksNone {
		t.Fatalf("no checks = %+v err=%v", none, err)
	}
}

// The merge is pinned to the completed head and never deletes the branch,
// bypasses protection, or schedules an auto-merge.
func TestGitHubMergePullRequestPinsHeadAfterReady(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh shell script is POSIX-only")
	}
	binDir := t.TempDir()
	log := filepath.Join(t.TempDir(), "gh.log")
	writeFakeGh(t, binDir, `#!/bin/sh
echo "$*" >> `+log+`
case "$1 $2" in
"pr view") echo '{"url":"`+ghMergeTestURL+`","state":"MERGED","headRefOid":"abc","mergeCommit":{"oid":"def"},"statusCheckRollup":[]}' ;;
esac
`)
	t.Setenv("PATH", binDir)
	state, err := NewProvider().MergePullRequest(context.Background(), port.IssueProviderPullRequestMergeRequest{
		URL: ghMergeTestURL, Method: model.RemoteMergeMethodSquash, HeadOID: "abc", MarkReady: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.State != "merged" || state.MergeCommitOID != "def" {
		t.Fatalf("readback = %+v", state)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"pr ready " + ghMergeTestURL,
		"pr merge " + ghMergeTestURL + " --squash --match-head-commit abc",
		"pr view " + ghMergeTestURL + " --json " + ghMergeStateFields,
	}
	if got := strings.Split(strings.TrimSpace(string(calls)), "\n"); !reflect.DeepEqual(got, want) {
		t.Fatalf("gh calls = %q, want %q", got, want)
	}
}

func TestGitHubMergePullRequestRequiresHead(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := NewProvider().MergePullRequest(context.Background(), port.IssueProviderPullRequestMergeRequest{URL: ghMergeTestURL, Method: model.RemoteMergeMethodSquash}); err == nil {
		t.Fatal("a merge without the expected head must be refused before calling gh")
	}
}
