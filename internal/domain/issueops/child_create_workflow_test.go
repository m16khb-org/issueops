package issueops

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

const childParentURL = "https://github.com/Org/Repo/issues/1"

const (
	opA = "0123456789abcdef0123456789abcdef"
	opB = "fedcba9876543210fedcba9876543210"
)

func TestValidateChildCreationReportsFirstMissingRequirement(t *testing.T) {
	preparedUmbrella := model.IssueOpsRecord{
		ID: "u", IssueURL: childParentURL, Branch: "9-umbrella",
		BranchPrepare: &model.IssueOpsBranchPrepare{Branch: "9-umbrella"},
	}
	unbranched := preparedUmbrella
	unbranched.Branch = ""
	cases := []struct {
		name      string
		record    model.IssueOpsRecord
		title     string
		labels    []string
		assignees []string
		want      string
	}{
		{"unlinked", model.IssueOpsRecord{IssueURL: " "}, "t", []string{"l"}, []string{"a"}, "cannot create child before linked parent issue"},
		{"umbrella gate", unbranched, "t", []string{"l"}, []string{"a"}, UmbrellaBranchGateReason(unbranched)},
		{"title", preparedUmbrella, "  ", []string{"l"}, []string{"a"}, "child title is required"},
		{"labels", preparedUmbrella, "t", nil, []string{"a"}, "at least one child label is required"},
		{"assignees", preparedUmbrella, "t", []string{"l"}, nil, "at least one child assignee is required"},
	}
	for _, tc := range cases {
		err := ValidateChildCreation(tc.record, tc.title, tc.labels, tc.assignees)
		if err == nil || err.Error() != tc.want {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	if err := ValidateChildCreation(preparedUmbrella, "t", []string{"l"}, []string{"a"}); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
}

func TestValidateCreatedChildRequiresVerifiedHierarchyAndURL(t *testing.T) {
	for _, tc := range []struct {
		verified bool
		url      string
		ok       bool
	}{{true, "https://x/1", true}, {false, "https://x/1", false}, {true, "  ", false}} {
		err := ValidateCreatedChild(tc.verified, tc.url)
		if (err == nil) != tc.ok {
			t.Errorf("(%v,%q) = %v", tc.verified, tc.url, err)
		}
	}
}

func TestChildRequestDigestIgnoresOrderAndWhitespaceButNotContent(t *testing.T) {
	base := ChildRequestDigest("github", "p", " title ", "\nbody\n", []string{"b", "a"}, []string{"y", "x"})
	if base != ChildRequestDigest("github", "p", "title", "body", []string{"a", "b"}, []string{"x", "y"}) {
		t.Fatal("digest depends on ordering or surrounding whitespace")
	}
	for name, other := range map[string]string{
		"provider": ChildRequestDigest("gitlab", "p", "title", "body", []string{"a", "b"}, []string{"x", "y"}),
		"labels":   ChildRequestDigest("github", "p", "title", "body", []string{"a"}, []string{"x", "y"}),
		"body":     ChildRequestDigest("github", "p", "title", "other", []string{"a", "b"}, []string{"x", "y"}),
	} {
		if other == base {
			t.Errorf("digest ignores %s", name)
		}
	}
	labels := []string{"b", "a"}
	ChildRequestDigest("github", "p", "t", "b", labels, nil)
	if labels[0] != "b" {
		t.Fatal("caller slice was sorted in place")
	}
}

func sealedChild(t *testing.T, record model.IssueOpsRecord, id, digest string) model.ChildCreateOperation {
	t.Helper()
	op, body, err := SealChildOperation(record, id, "explicit", "github", " Child ", " body ", digest, "t0", []string{"l"}, []string{"a"})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if !strings.HasSuffix(body, "\n\n<!-- issueops:child-create:"+id+" -->") || !strings.HasPrefix(body, "body\n") {
		t.Fatalf("body = %q", body)
	}
	return op
}

func TestSealChildOperationDerivesProjectAuthorityFromParent(t *testing.T) {
	record := model.IssueOpsRecord{IssueURL: childParentURL}
	op := sealedChild(t, record, opA, dig("d1"))
	if op.ProjectAuthority != "github.com/org/repo" || op.Title != "Child" || op.Status != model.IssueCreateIntentPending ||
		op.Attempt != 1 || op.ParentURL != childParentURL || op.Generation != 0 || op.Holder != nil {
		t.Fatalf("unexpected op: %+v", op)
	}
	if _, _, err := SealChildOperation(record, opA, "explicit", "gitlab", "t", "b", "d", "t0", []string{"l"}, []string{"a"}); err == nil ||
		err.Error() != "invalid gitlab child URL" {
		t.Fatalf("provider/URL mismatch error = %v", err)
	}
	bad := model.IssueOpsRecord{IssueURL: "http://github.com/Org/Repo/issues/1"}
	if _, _, err := SealChildOperation(bad, opA, "explicit", "github", "t", "b", "d", "t0", nil, nil); err == nil ||
		err.Error() != "invalid child project URL" {
		t.Fatalf("non-https parent error = %v", err)
	}
}

func TestChildProjectAuthorityRejectsMalformedIssueURLs(t *testing.T) {
	cases := []struct {
		raw, provider, want string
	}{
		{"https://github.com/o/r/issues/7", "github", "github.com/o/r"},
		{"https://GitHub.com:443/O/R/issues/7", "github", "github.com/o/r"},
		{"https://gitlab.example.com/g/sub/p/-/issues/7", "gitlab", "gitlab.example.com/g/sub/p"},
		{"https://gitlab.example.com/g/p/-/work_items/7", "gitlab", "gitlab.example.com/g/p"},
		{"https://github.com/o/r/pull/7", "github", ""},
		{"https://gitlab.com/o/r/issues/7", "gitlab", ""},
		{"https://github.com/o/r/issues/7", "gitlab", ""},
		{"https://github.com/o/r/issues/7", "bitbucket", ""},
		{"https://github.com/o/r/issues/+7", "github", ""},
		{"https://github.com/o/r/issues/x", "github", ""},
		{"https://github.com/../r/issues/7", "github", ""},
		{"https://github.com/o/r/issues/7?x=1", "github", ""},
		{"https://u@github.com/o/r/issues/7", "github", ""},
		{"https://github.com/o/r/issues/7#f", "github", ""},
		{"://bad", "github", ""},
	}
	for _, tc := range cases {
		got, err := childProjectAuthority(tc.raw, tc.provider)
		if tc.want == "" {
			if err == nil {
				t.Errorf("%s (%s) accepted as %q", tc.raw, tc.provider, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("%s = %q, %v; want %q", tc.raw, got, err, tc.want)
		}
	}
}

func TestChildOperationLifecycleBeginRecordComplete(t *testing.T) {
	record := model.IssueOpsRecord{IssueURL: childParentURL}
	op := sealedChild(t, record, opA, dig("d1"))

	record, begun, started, err := BeginChildOperation(record, op, opA)
	if err != nil || !started || begun.OperationID != opA || len(record.ChildCreateOperations) != 1 || record.UpdatedAt != "t0" {
		t.Fatalf("begin: %+v %v %v", record, started, err)
	}
	found, ok := FindChildOperation(record, opA, "")
	if !ok || found.RequestSHA256 != dig("d1") {
		t.Fatalf("explicit lookup failed: %+v %v", found, ok)
	}
	if _, ok := FindChildOperation(record, "nope", ""); ok {
		t.Fatal("unknown id found")
	}

	// 같은 id의 pending 작업을 다시 시작하면 해결 전까지 거부한다.
	if _, _, _, err := BeginChildOperation(record, op, opA); err == nil || err.Error() != "unresolved child operation; reconcile before creating" {
		t.Fatalf("restart pending = %v", err)
	}
	// 다른 요청 본문은 거부한다.
	mismatch := op
	mismatch.RequestSHA256 = dig("other")
	if _, _, _, err := BeginChildOperation(record, mismatch, opA); err == nil || err.Error() != "child operation payload mismatch" {
		t.Fatalf("payload mismatch = %v", err)
	}
	// 미해결 작업이 있으면 다른 id의 새 작업도 거부한다.
	other := sealedChild(t, record, opB, dig("d2"))
	if _, _, _, err := BeginChildOperation(record, other, opB); err == nil || err.Error() != "unresolved child operation; reconcile before creating" {
		t.Fatalf("second concurrent op = %v", err)
	}

	if _, err := RecordChildOutcome(record, "missing", model.IssueCreateIntentCompleted, "", "", "t1"); err == nil || err.Error() != "child operation not found" {
		t.Fatalf("missing op = %v", err)
	}
	for name, url := range map[string]string{
		"other project": "https://github.com/Org/Other/issues/2",
		"parent url":    childParentURL,
	} {
		if _, err := RecordChildOutcome(record, opA, model.IssueCreateIntentURLObserved, url, "", "t1"); err == nil {
			t.Errorf("%s accepted as child URL", name)
		}
	}

	childURL := "https://github.com/Org/Repo/issues/2"
	done, err := CompleteChildOperation(record, opA, childURL, "t2")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	completed, _ := FindChildOperation(done, opA, "")
	if completed.Status != model.IssueCreateIntentCompleted || completed.CanonicalURL != childURL || completed.UpdatedAt != "t2" {
		t.Fatalf("completed op = %+v", completed)
	}
	links := 0
	for _, link := range done.IssueLinks {
		if link.Type == "child" && link.URL == childURL && link.Title == "Child" {
			links++
		}
	}
	if links != 1 {
		t.Fatalf("child link count = %d in %+v", links, done.IssueLinks)
	}
	if record.ChildCreateOperations[0].Status != model.IssueCreateIntentPending {
		t.Fatal("input record mutated")
	}

	// 완료된 작업은 재시도해도 새로 시작하지 않고 그대로 돌려준다.
	again, replay, started, err := BeginChildOperation(done, op, opA)
	if err != nil || started || replay.Status != model.IssueCreateIntentCompleted || len(again.ChildCreateOperations) != 1 {
		t.Fatalf("replay completed: %+v %v %v", replay, started, err)
	}
	// 완료 후에는 다른 id의 새 작업을 시작할 수 있다.
	if _, _, started, err := BeginChildOperation(done, other, opB); err != nil || !started {
		t.Fatalf("next op after completion: %v %v", started, err)
	}
	// 완료된 작업을 다시 완료 처리하는 것은 전이 규칙이 거부한다.
	if _, err := CompleteChildOperation(done, opA, childURL, "t3"); err == nil || !strings.Contains(err.Error(), "completed -> completed") {
		t.Fatalf("completing twice = %v", err)
	}
	// 이미 연결된 child 링크가 있으면 중복 링크를 만들지 않는다.
	linked := record
	linked.IssueLinks = []model.IssueOpsIssueLink{{Type: "child", URL: childURL}}
	deduped, err := CompleteChildOperation(linked, opA, childURL, "t4")
	if err != nil {
		t.Fatalf("complete with existing link: %v", err)
	}
	n := 0
	for _, link := range deduped.IssueLinks {
		if link.Type == "child" && link.URL == childURL {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("child links = %d, want 1", n)
	}
}

func TestBeginChildOperationRetriesNotInvokedOperation(t *testing.T) {
	record := model.IssueOpsRecord{IssueURL: childParentURL}
	op := sealedChild(t, record, opA, dig("d1"))
	op.Status = model.IssueCreateIntentNotInvoked
	op.Failure = "earlier"
	record.ChildCreateOperations = []model.ChildCreateOperation{op}

	got, retry, started, err := BeginChildOperation(record, op, opA)
	if err != nil || !started {
		t.Fatalf("retry: %v %v", started, err)
	}
	if retry.Status != model.IssueCreateIntentPending || retry.Failure != "" || retry.Attempt != op.Attempt+1 ||
		got.ChildCreateOperations[0].Attempt != op.Attempt+1 {
		t.Fatalf("retry not reset: %+v", retry)
	}
	// 암묵적(origin=implicit) 작업은 id 없이 요청 digest로 찾는다.
	implicit := op
	implicit.Origin, implicit.RequestSHA256 = "implicit", dig("dig")
	record.ChildCreateOperations = []model.ChildCreateOperation{implicit}
	if found, ok := FindChildOperation(record, "", dig("dig")); !ok || found.OperationID != opA {
		t.Fatalf("implicit lookup = %+v %v", found, ok)
	}
	if _, ok := FindChildOperation(record, "", dig("other")); ok {
		t.Fatal("digest mismatch found an operation")
	}
}

func TestValidateChildOperationAuthorityTracksParentAndExecutionBinding(t *testing.T) {
	record := model.IssueOpsRecord{IssueURL: childParentURL}
	op := sealedChild(t, record, opA, dig("d1"))
	if err := ValidateChildOperationAuthority(record, op); err != nil {
		t.Fatalf("unbound op: %v", err)
	}
	moved := record
	moved.IssueURL = "https://github.com/Org/Repo/issues/5"
	if err := ValidateChildOperationAuthority(moved, op); err == nil || err.Error() != "child operation parent/project changed" {
		t.Fatalf("parent moved = %v", err)
	}
	stale := op
	stale.Generation = 3
	if err := ValidateChildOperationAuthority(record, stale); err == nil || err.Error() != "child operation authority changed" {
		t.Fatalf("generation without execution = %v", err)
	}
	withExec := record
	withExec.Execution = &model.Execution{}
	withExec.Execution.Lease.Generation = 2
	withExec.Execution.Workspace.Root = "/w"
	if err := ValidateChildOperationAuthority(withExec, op); err == nil || !strings.Contains(err.Error(), "explicit reconcile required") {
		t.Fatalf("rebound execution = %v", err)
	}

	// Rebind는 현재 실행 권한으로 작업을 다시 묶는다.
	withExec.ChildCreateOperations = []model.ChildCreateOperation{op}
	rebound, err := RebindChildOperation(withExec, opA)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := FindChildOperation(rebound, opA, "")
	if got.Generation != 2 || got.CWD != "/w" {
		t.Fatalf("rebind = %+v", got)
	}
	if err := ValidateChildOperationAuthority(rebound, got); err != nil {
		t.Fatalf("authority after rebind: %v", err)
	}
	if _, err := RebindChildOperation(withExec, "missing"); err == nil || err.Error() != "child operation not found" {
		t.Fatalf("rebind missing = %v", err)
	}
	withExec.IssueURL = "https://github.com/Org/Repo/issues/9"
	if _, err := RebindChildOperation(withExec, opA); err == nil || err.Error() != "child operation parent changed" {
		t.Fatalf("rebind moved parent = %v", err)
	}
}

// dig는 테스트용 요청 digest(유효한 sha256 hex)를 만든다.
func dig(seed string) string {
	return ChildRequestDigest("github", childParentURL, seed, "", nil, nil)
}
