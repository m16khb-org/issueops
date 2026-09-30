package issueops

import (
	"issueops/internal/contract/issueops"
	"strings"
	"testing"
)

// parseGitLabExecutionIssueIdentity는 실행 스냅샷 신원 검증의 근간이다.
// 정식 HTTPS URL만 받아들이고, 프로젝트 경로/IID 정규형을 강제한다.
func TestParseGitLabExecutionIssueIdentity(t *testing.T) {
	valid := map[string]gitLabExecutionIssueIdentity{
		"https://gitlab.com/acme/repo/-/issues/42":    {authority: "gitlab.com", project: "acme/repo", iid: "42"},
		"https://GitLab.com/acme/repo/-/work_items/7": {authority: "gitlab.com", project: "acme/repo", iid: "7"},
		"https://gl.example.co.kr/a/b/c/-/issues/1":   {authority: "gl.example.co.kr", project: "a/b/c", iid: "1"},
	}
	for raw, want := range valid {
		got, err := parseGitLabExecutionIssueIdentity(raw)
		if err != nil || got != want {
			t.Fatalf("parse(%q) = %+v, %v want %+v", raw, got, err, want)
		}
	}
	invalid := []string{
		"", "  https://gitlab.com/a/b/-/issues/1  ",
		"http://gitlab.com/a/b/-/issues/1",
		"https://gitlab.com/a/b/-/issues/1?x=1",
		"https://gitlab.com/a/b/-/issues/1#frag",
		"https://user@gitlab.com/a/b/-/issues/1",
		"https://gitlab.com/a/b/-/merge_requests/1",
		"https://gitlab.com/a/b/-/issues/0",
		"https://gitlab.com/a/b/-/issues/007",
		"https://gitlab.com/a/b/-/issues/1.0",
		"https://gitlab.com/-/issues/1",
		"https://gitlab.com/a/../b/-/issues/1",
		"https://gitlab.com/a/b/-/issues/1/",
		// 인코딩된 슬래시(%2F)는 디코딩 결과가 경로 구분자라 프로젝트
		// 경로 정규형 위반으로 거부된다(fail-closed 계약).
		"https://gitlab.com/acme/re%2Fpo/-/issues/9",
	}
	for _, raw := range invalid {
		if _, err := parseGitLabExecutionIssueIdentity(raw); err == nil {
			t.Fatalf("parse(%q) must fail closed", raw)
		}
	}
}

func TestSameGitLabExecutionIssueIdentity(t *testing.T) {
	base := "https://gitlab.com/acme/repo/-/issues/42"
	if !SameGitLabExecutionIssueIdentity(base, "https://gitlab.com/acme/repo/-/issues/42") {
		t.Fatal("identical identities must match")
	}
	if SameGitLabExecutionIssueIdentity(base, "https://gitlab.com/acme/repo/-/issues/43") ||
		SameGitLabExecutionIssueIdentity(base, "https://other.com/acme/repo/-/issues/42") ||
		SameGitLabExecutionIssueIdentity(base, "https://gitlab.com/acme/other/-/issues/42") ||
		SameGitLabExecutionIssueIdentity(base, "not-a-url") ||
		SameGitLabExecutionIssueIdentity("broken", base) {
		t.Fatal("distinct or invalid identities must not match")
	}
}

func TestValidateGitLabExecutionIssueSnapshot(t *testing.T) {
	base := "https://gitlab.com/acme/repo/-/issues/42"
	if err := ValidateGitLabExecutionSnapshot(base, base, "설명", "opened"); err != nil {
		t.Fatalf("valid snapshot must pass: %v", err)
	}
	cases := []struct {
		name string
		snap struct{ URL, Body, State string }
	}{
		{"url mismatch", struct{ URL, Body, State string }{URL: "https://gitlab.com/acme/repo/-/issues/43", Body: "b", State: "opened"}},
		{"empty body", struct{ URL, Body, State string }{URL: base, Body: "  ", State: "opened"}},
		{"oversized body", struct{ URL, Body, State string }{URL: base, Body: strings.Repeat("x", executionIssueSnapshotBodyLimit+1), State: "opened"}},
		{"bad state", struct{ URL, Body, State string }{URL: base, Body: "b", State: "merged"}},
	}
	for _, tc := range cases {
		if err := ValidateGitLabExecutionSnapshot(base, tc.snap.URL, tc.snap.Body, tc.snap.State); err == nil {
			t.Fatalf("%s must fail closed", tc.name)
		}
	}
}

func TestValidateExecutionIssueSnapshotActionAndRecord(t *testing.T) {
	for _, action := range []string{string(issueops.ExecutionActionPrepare), string(issueops.ExecutionActionClaim)} {
		if err := ValidateExecutionSnapshotAction(issueops.ExecutionActionRequest{Action: action}); err != nil {
			t.Fatalf("action %s must pass: %v", action, err)
		}
	}
	if err := ValidateExecutionSnapshotAction(issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionReconcile, Confirm: true, Preview: false,
	}); err != nil {
		t.Fatalf("confirm reconcile must pass: %v", err)
	}
	if err := ValidateExecutionSnapshotAction(issueops.ExecutionActionRequest{Action: issueops.ExecutionActionReconcile}); err == nil {
		t.Fatal("preview reconcile must fail")
	}
	if err := ValidateExecutionSnapshotAction(issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionReplace, ReplaceAction: issueops.ExecutionReplaceFinalize,
	}); err != nil {
		t.Fatalf("replace finalize must pass: %v", err)
	}
	if err := ValidateExecutionSnapshotAction(issueops.ExecutionActionRequest{Action: issueops.ExecutionActionRelease}); err == nil {
		t.Fatal("release must fail closed")
	}

	// reconcile은 orca pending worktree_create에서만 허용된다.
	if err := ValidateExecutionSnapshotRecord(
		issueops.ExecutionActionRequest{Action: issueops.ExecutionActionReconcile, Confirm: true},
		issueops.IssueOpsRecord{Execution: &issueops.Execution{
			Mode:    issueops.ExecutionModeOrca,
			Pending: &issueops.ExternalIntent{Kind: "worktree_create"},
		}},
	); err != nil {
		t.Fatalf("orca pending record must pass: %v", err)
	}
	for name, record := range map[string]issueops.IssueOpsRecord{
		"direct mode": {Execution: &issueops.Execution{Mode: issueops.ExecutionModeDirect, Pending: &issueops.ExternalIntent{Kind: "worktree_create"}}},
		"no pending":  {Execution: &issueops.Execution{Mode: issueops.ExecutionModeOrca}},
		"other kind":  {Execution: &issueops.Execution{Mode: issueops.ExecutionModeOrca, Pending: &issueops.ExternalIntent{Kind: "gh_run"}}},
	} {
		if err := ValidateExecutionSnapshotRecord(
			issueops.ExecutionActionRequest{Action: issueops.ExecutionActionReconcile, Confirm: true}, record,
		); err == nil {
			t.Fatalf("%s must fail closed", name)
		}
	}
}
