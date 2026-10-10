package gitlab

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

const glMergeTestURL = "https://gitlab.example/group/repo/-/merge_requests/16"

func TestGitLabMergeStateMapsPipelineAndMergeStatus(t *testing.T) {
	state, title, err := parseGlabMergeState([]byte(`{"web_url":"` + glMergeTestURL + `","title":"Draft: fix","state":"opened","draft":true,"sha":"abc","target_branch":"main","has_conflicts":false,"detailed_merge_status":"not_approved","head_pipeline":{"id":7,"status":"running"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if title != "Draft: fix" || state.State != "open" || !state.Draft || state.Checks != model.RemoteChecksPending || state.Blocked != "not_approved" || state.Conflict {
		t.Fatalf("state = %+v", state)
	}
	if !reflect.DeepEqual(state.Pending, []string{"pipeline #7"}) {
		t.Fatalf("pending = %v", state.Pending)
	}
	// ci_still_running and draft_status are already covered by checks and draft;
	// they must not add a second, provider-level block.
	for _, status := range []string{"ci_still_running", "draft_status", "mergeable", "checking"} {
		state, _, err := parseGlabMergeState([]byte(`{"state":"opened","detailed_merge_status":"` + status + `"}`))
		if err != nil || state.Blocked != "" || state.Checks != model.RemoteChecksNone {
			t.Fatalf("%s: %+v err=%v", status, state, err)
		}
	}
	failed, _, err := parseGlabMergeState([]byte(`{"state":"opened","detailed_merge_status":"conflict","head_pipeline":{"id":8,"status":"failed"}}`))
	if err != nil || !failed.Conflict || failed.Checks != model.RemoteChecksFailing {
		t.Fatalf("failed = %+v err=%v", failed, err)
	}
}

// Ready clears the title prefix, and the merge pins the head with auto-merge
// and source-branch removal explicitly off.
func TestGitLabMergeRequestClearsDraftAndPinsHead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake glab shell script is POSIX-only")
	}
	binDir := t.TempDir()
	dir := t.TempDir()
	log := filepath.Join(dir, "glab.log")
	merged := filepath.Join(dir, "merged")
	writeFakeGlab(t, binDir, `#!/bin/sh
echo "$*" >> `+log+`
case "$*" in
*"/merge --hostname"*) : > `+merged+`; echo '{}'; exit 0 ;;
*"--method PUT"*) echo '{}'; exit 0 ;;
esac
if [ -f `+merged+` ]; then echo '{"web_url":"`+glMergeTestURL+`","state":"merged","sha":"abc","squash_commit_sha":"def"}'; exit 0; fi
echo '{"web_url":"`+glMergeTestURL+`","title":"Draft: fix login","state":"opened","draft":true,"sha":"abc"}'
`)
	t.Setenv("PATH", binDir)
	state, err := NewProvider().MergePullRequest(context.Background(), port.IssueProviderPullRequestMergeRequest{
		URL: glMergeTestURL, Method: model.RemoteMergeMethodSquash, HeadOID: "abc", MarkReady: true,
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
	endpoint := "api projects/group%2Frepo/merge_requests/16 --hostname gitlab.example"
	want := []string{
		endpoint,
		endpoint + " --method PUT -f title=fix login",
		"api projects/group%2Frepo/merge_requests/16/merge --hostname gitlab.example --method PUT -f sha=abc -F squash=true -F should_remove_source_branch=false -F auto_merge=false",
		endpoint,
	}
	if got := strings.Split(strings.TrimSpace(string(calls)), "\n"); !reflect.DeepEqual(got, want) {
		t.Fatalf("glab calls = %q, want %q", got, want)
	}
}

func TestGitLabMergeRequestRejectsRebaseMethod(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := NewProvider().MergePullRequest(context.Background(), port.IssueProviderPullRequestMergeRequest{URL: glMergeTestURL, Method: model.RemoteMergeMethodRebase, HeadOID: "abc"})
	if err == nil || !strings.Contains(err.Error(), "per project") {
		t.Fatalf("rebase must be refused before any glab call: %v", err)
	}
}
