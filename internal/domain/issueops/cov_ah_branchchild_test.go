package issueops

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
)

func TestCovBranchChildSelectUmbrellaCycle(t *testing.T) {
	child := "https://github.com/Org/Repo/issues/2"
	link := []model.IssueOpsIssueLink{{Type: "child", URL: " " + child + " "}}
	done := model.IssueOpsRecord{ID: "done", Phase: model.IssueOpsPhaseDone, IssueLinks: link}
	related := model.IssueOpsRecord{ID: "related", IssueLinks: []model.IssueOpsIssueLink{{Type: "related", URL: child}}}
	other := model.IssueOpsRecord{ID: "other", IssueLinks: []model.IssueOpsIssueLink{{Type: "child", URL: child + "0"}}}
	hit := model.IssueOpsRecord{ID: "hit", Phase: model.IssueOpsPhaseImplement, IssueLinks: link}
	matches := []CyclePathMatch{
		{Record: hit, Repository: false},
		{Record: done, Repository: true},
		{Record: related, Repository: true},
		{Record: other, Repository: true},
		{Record: hit, Repository: true, Worktree: true},
	}
	got, ok := SelectUmbrellaCycle(matches, " "+child+" ")
	if !ok || got.ID != "hit" {
		t.Fatalf("got %q %v", got.ID, ok)
	}
	if _, ok := SelectUmbrellaCycle(matches[:4], child); ok {
		t.Fatal("non-repository, done, non-child and other-url records must not match")
	}
	if _, ok := SelectUmbrellaCycle(matches, "  "); ok {
		t.Fatal("blank child URL must not match")
	}
}

func TestCovBranchChildAwaitBranchLinkGates(t *testing.T) {
	good := model.IssueOpsRecord{BranchPrepare: &model.IssueOpsBranchPrepare{Provider: "github", IssueURL: "u", Branch: "1-a", BaseSHA: "abc"}}
	if got := AwaitBranchLinkGates(good, true); len(got) != 0 {
		t.Fatalf("good = %v", got)
	}
	if got := AwaitBranchLinkGates(model.IssueOpsRecord{}, false); !slices.Equal(got, []string{"linked_branch_observation_unavailable", "branch_prepare_missing"}) {
		t.Fatalf("missing prepare = %v", got)
	}
	bad := model.IssueOpsRecord{BranchPrepare: &model.IssueOpsBranchPrepare{Provider: "gitlab", IssueURL: " ", Branch: " ", BaseSHA: " "}}
	want := []string{"branch_await_link_is_github_only", "issue_url_missing", "branch_missing", "sealed_base_missing"}
	if got := AwaitBranchLinkGates(bad, true); !slices.Equal(got, want) {
		t.Fatalf("bad = %v", got)
	}
}

func TestCovBranchChildAwaitBranchLinkTimeout(t *testing.T) {
	cases := []struct {
		raw  string
		want time.Duration
		err  string
	}{
		{"", 10 * time.Minute, ""},
		{"  ", 10 * time.Minute, ""},
		{" 90s ", 90 * time.Second, ""},
		{"30m", 30 * time.Minute, ""},
		{"31m", 0, "must be between 0 and 30m0s, got 31m0s"},
		{"0s", 0, "must be between 0"},
		{"-1m", 0, "must be between 0"},
		{"soon", 0, `is not a duration: "soon"`},
	}
	for _, tc := range cases {
		got, err := AwaitBranchLinkTimeout(tc.raw)
		if tc.err == "" {
			if err != nil || got != tc.want {
				t.Fatalf("%q = %v, %v", tc.raw, got, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.err) || got != 0 {
			t.Fatalf("%q = %v, %v", tc.raw, got, err)
		}
	}
}

func TestCovBranchChildVerdictPreparation(t *testing.T) {
	reason, ev, err := PrepareChildVerdict("accepted", "ignored reason text", []string{" e1 ", " "})
	if err != nil || reason != "" || !slices.Equal(ev, []string{"e1"}) {
		t.Fatalf("accepted = %q %v %v", reason, ev, err)
	}
	if _, _, err := PrepareChildVerdict("accepted", "", []string{" "}); err == nil || err.Error() != "validation_evidence is required" {
		t.Fatalf("accepted w/o evidence = %v", err)
	}
	if _, _, err := PrepareChildVerdict("rejected", "short", nil); err == nil || err.Error() != "reason must be at least 10 characters" {
		t.Fatalf("short reason = %v", err)
	}
	reason, ev, err = PrepareChildVerdict("dropped", "  not needed anymore ", []string{"x"})
	if err != nil || reason != "not needed anymore" || ev != nil {
		t.Fatalf("dropped = %q %v %v", reason, ev, err)
	}
	reason, ev, err = PrepareChildVerdict("rejected", "does not meet criteria", []string{"x"})
	if err != nil || reason != "does not meet criteria" || !slices.Equal(ev, []string{"x"}) {
		t.Fatalf("rejected = %q %v %v", reason, ev, err)
	}
}

func TestCovBranchChildVerdictValidation(t *testing.T) {
	if err := ValidateChildValidationIDs("", "c"); err == nil || err.Error() != "parent_id is required" {
		t.Fatalf("parent = %v", err)
	}
	if err := ValidateChildValidationIDs("p", ""); err == nil || err.Error() != "child_id is required" {
		t.Fatalf("child = %v", err)
	}
	if err := ValidateChildValidationIDs("p", "c"); err != nil {
		t.Fatal(err)
	}
	owned := model.IssueOpsRecord{ID: "c", Phase: model.IssueOpsPhaseImplement, Delegation: &model.IssueOpsDelegationContract{ParentCycleID: " p "}}
	if err := ValidateChildForVerdict(model.IssueOpsRecord{ID: "c"}, "p", "dropped"); err == nil || err.Error() != "child_parent_mismatch: c" {
		t.Fatalf("no delegation = %v", err)
	}
	if err := ValidateChildForVerdict(owned, "q", "dropped"); err == nil || err.Error() != "child_parent_mismatch: c" {
		t.Fatalf("other parent = %v", err)
	}
	if err := ValidateChildForVerdict(owned, "p", "accepted"); err == nil || err.Error() != "child_not_done: c" {
		t.Fatalf("accepted not done = %v", err)
	}
	if err := ValidateChildForVerdict(owned, "p", "dropped"); err != nil {
		t.Fatalf("dropped unfinished child = %v", err)
	}
	owned.Phase = model.IssueOpsPhaseDone
	if err := ValidateChildForVerdict(owned, "p", "accepted"); err != nil {
		t.Fatalf("accepted done child = %v", err)
	}
	// 보관된(archived) child가 다시 나타나면 parent 일치와 무관하게 항상 오류다.
	if err := ValidateArchivedChildObservation(owned, "p"); err == nil || err.Error() != "child_reappeared: c" {
		t.Fatalf("reappeared = %v", err)
	}
	if err := ValidateArchivedChildObservation(owned, "q"); err == nil || err.Error() != "child_parent_mismatch: c" {
		t.Fatalf("reappeared other parent = %v", err)
	}
}

func TestCovBranchChildApplyVerdict(t *testing.T) {
	child := model.IssueOpsRecord{ID: "c1", Repo: "r", Branch: "2-c", Phase: model.IssueOpsPhaseDone}
	parent := model.IssueOpsRecord{ID: "p", Repo: " r "}
	updated, ref, err := ApplyChildVerdict(parent, child, "accepted", "", []string{"ev"}, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if ref.CycleID != "c1" || ref.ValidationVerdict != "accepted" || ref.ValidatedAt != "t1" || !slices.Equal(ref.ValidationEvidence, []string{"ev"}) {
		t.Fatalf("ref = %+v", ref)
	}
	if len(updated.ChildCycles) != 1 || len(parent.ChildCycles) != 0 {
		t.Fatalf("parent mutated or not indexed: %d %d", len(updated.ChildCycles), len(parent.ChildCycles))
	}

	if _, _, err := ApplyChildVerdict(parent, model.IssueOpsRecord{ID: "c2", Repo: "other"}, "dropped", "long enough reason", nil, "t"); err == nil || err.Error() != "child_repo_mismatch: c2" {
		t.Fatalf("repo mismatch = %v", err)
	}

	// 이미 index된 child는 기존 ref를 덮어쓴다(중복 추가 금지).
	again, ref2, err := ApplyChildVerdict(updated, child, "dropped", "no longer needed", []string{"ignored"}, "t2")
	if err != nil {
		t.Fatal(err)
	}
	if len(again.ChildCycles) != 1 || again.ChildCycles[0].ValidationVerdict != "dropped" || ref2.ValidatedAt != "t2" {
		t.Fatalf("overwrite = %+v", again.ChildCycles)
	}
	if updated.ChildCycles[0].ValidationVerdict != "accepted" {
		t.Fatal("input parent's index was mutated")
	}

	if _, _, err := ApplyArchivedChildVerdict(updated, "ghost", "accepted", "", []string{"e"}, "t"); err == nil || err.Error() != "child_not_indexed: ghost" {
		t.Fatalf("not indexed = %v", err)
	}
}

func TestCovBranchChildStartPreconditions(t *testing.T) {
	ok := model.IssueOpsRecord{
		Branch: "1-parent", Phase: model.IssueOpsPhaseImplement,
		ArchitectureReview:   &reviewcontract.ArchitectureReview{Approved: true},
		CompatibilityReview:  &reviewcontract.CompatibilityReview{Approved: true, Blockers: []string{" "}},
		DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{RecordedAt: "t", Verdict: "proceed"},
	}
	req := model.IssueOpsChildStartRequest{Branch: "2-child"}
	if got := ChildStartMissingPreconditions(ok, req); len(got) != 0 {
		t.Fatalf("ok = %v", got)
	}
	bad := model.IssueOpsRecord{Branch: "1-parent", Delegation: &model.IssueOpsDelegationContract{}}
	want := []string{"parent_phase_not_implement", "parent_architecture_review_unapproved", "parent_compatibility_unapproved", "parent_devils_advocate_missing", "delegation_depth_exceeded", "child_branch_equals_parent"}
	if got := ChildStartMissingPreconditions(bad, model.IssueOpsChildStartRequest{Branch: " 1-parent "}); !slices.Equal(got, want) {
		t.Fatalf("bad = %v", got)
	}
	blocked := ok
	blocked.CompatibilityReview = &reviewcontract.CompatibilityReview{Approved: true, Blockers: []string{"b"}}
	if got := ChildStartMissingPreconditions(blocked, req); !slices.Equal(got, []string{"parent_compatibility_unapproved"}) {
		t.Fatalf("blocker = %v", got)
	}
	for _, verdict := range []string{"stop", "revise"} {
		stopped := ok
		stopped.DevilsAdvocateReview = &model.IssueOpsDevilsAdvocateReview{RecordedAt: "t", Verdict: verdict}
		if got := ChildStartMissingPreconditions(stopped, req); !slices.Equal(got, []string{"parent_devils_advocate_missing"}) {
			t.Fatalf("%s = %v", verdict, got)
		}
		stopped.DevilsAdvocateReview.Waived = true
		if got := ChildStartMissingPreconditions(stopped, req); len(got) != 0 {
			t.Fatalf("waived %s = %v", verdict, got)
		}
	}
	unrecorded := ok
	unrecorded.DevilsAdvocateReview = &model.IssueOpsDevilsAdvocateReview{RecordedAt: " ", Verdict: "proceed"}
	if got := ChildStartMissingPreconditions(unrecorded, req); !slices.Equal(got, []string{"parent_devils_advocate_missing"}) {
		t.Fatalf("unrecorded = %v", got)
	}
}

func TestCovBranchChildPrepareChildStart(t *testing.T) {
	full := model.IssueOpsChildStartRequest{ParentID: " p ", Branch: " 2-c ", Title: " T ", TaskScope: " scope ", ParentPlanPath: " plan.md ", ChildIssueURL: " u ", AcceptanceCriteria: []string{" ok "}}
	got, err := PrepareChildStart(full)
	if err != nil {
		t.Fatal(err)
	}
	if got.ParentID != "p" || got.Branch != "2-c" || got.Title != "T" || got.TaskScope != "scope" || got.ParentPlanPath != "plan.md" || got.ChildIssueURL != "u" {
		t.Fatalf("not trimmed: %+v", got)
	}
	cases := []struct {
		mutate func(*model.IssueOpsChildStartRequest)
		want   string
	}{
		{func(r *model.IssueOpsChildStartRequest) { r.ParentID = " " }, "parent_id is required"},
		{func(r *model.IssueOpsChildStartRequest) { r.Branch = " " }, "branch is required"},
		{func(r *model.IssueOpsChildStartRequest) { r.TaskScope = " " }, "task_scope is required"},
		{func(r *model.IssueOpsChildStartRequest) { r.AcceptanceCriteria = []string{" "} }, "acceptance_criteria requires at least one entry"},
	}
	for _, tc := range cases {
		req := full
		tc.mutate(&req)
		if _, err := PrepareChildStart(req); err == nil || err.Error() != tc.want {
			t.Fatalf("want %q got %v", tc.want, err)
		}
	}
}

func TestCovBranchChildValidateChildMutation(t *testing.T) {
	if err := ValidateChildMutation(model.IssueOpsRecord{}); err != nil {
		t.Fatal(err)
	}
	attempt := model.IssueOpsRecord{CleanupAttempt: &model.IssueOpsCleanupAttempt{Operation: "finish"}}
	if err := ValidateChildMutation(attempt); err == nil || err.Error() != "cleanup finish apply is in progress" {
		t.Fatalf("attempt = %v", err)
	}
	applying := model.IssueOpsRecord{CleanupAbandonFailure: &model.IssueOpsCleanupAbandonFailure{Step: "applying"}}
	if err := ValidateChildMutation(applying); err == nil || err.Error() != "cleanup abandon apply is in progress" {
		t.Fatalf("applying = %v", err)
	}
	failed := model.IssueOpsRecord{CleanupAbandonFailure: &model.IssueOpsCleanupAbandonFailure{Step: "verifying"}}
	if err := ValidateChildMutation(failed); err != nil {
		t.Fatalf("non-applying failure blocked: %v", err)
	}
}

func TestCovBranchChildRegisterChildReference(t *testing.T) {
	req := model.IssueOpsChildStartRequest{Title: " T ", ChildIssueURL: " u "}
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	child := model.IssueOpsRecord{ID: "c", Branch: "2-c", CreatedAt: old.Add(time.Hour).Format(time.RFC3339Nano)}
	parent := model.IssueOpsRecord{ChildCycles: []model.IssueOpsChildCycleRef{{CycleID: "x"}}}

	added, ref, changed := RegisterChildReference(parent, child, req, "now")
	if !changed || ref.CycleID != "c" || ref.Title != "T" || ref.ChildIssueURL != "u" || ref.CreatedAt != "now" || len(added.ChildCycles) != 2 || len(parent.ChildCycles) != 1 {
		t.Fatalf("append = %+v %+v %v", added.ChildCycles, ref, changed)
	}

	existing := model.IssueOpsChildCycleRef{CycleID: "c", Branch: "old", CreatedAt: old.Format(time.RFC3339Nano)}
	parent.ChildCycles = []model.IssueOpsChildCycleRef{{CycleID: "x"}, existing}
	replaced, ref, changed := RegisterChildReference(parent, child, req, "now")
	if !changed || ref.Branch != "2-c" || replaced.ChildCycles[1].Branch != "2-c" || parent.ChildCycles[1].Branch != "old" {
		t.Fatalf("newer child must replace: %+v %v", replaced.ChildCycles, changed)
	}

	stale := child
	stale.CreatedAt = old.Add(-time.Hour).Format(time.RFC3339Nano)
	kept, ref, changed := RegisterChildReference(parent, stale, req, "now")
	if changed || !reflect.DeepEqual(ref, existing) || !reflect.DeepEqual(kept.ChildCycles, parent.ChildCycles) {
		t.Fatalf("older child must keep existing: %+v %v", kept.ChildCycles, changed)
	}
	unparsable := child
	unparsable.CreatedAt = "garbage"
	if _, ref, changed := RegisterChildReference(parent, unparsable, req, "now"); changed || ref.Branch != "old" {
		t.Fatalf("unparsable timestamp must keep existing: %+v %v", ref, changed)
	}
}

func covBranchChildExecution(generation uint64, root string) *model.Execution {
	e := &model.Execution{}
	e.Lease.Generation = generation
	e.Lease.Holder = &model.NativeActor{Host: "h", SessionID: "s"}
	e.Workspace.Root = root
	return e
}

func TestCovBranchChildSealBindsExecutionAuthority(t *testing.T) {
	record := model.IssueOpsRecord{IssueURL: childParentURL, Execution: covBranchChildExecution(4, "/w")}
	op, _, err := SealChildOperation(record, opA, "explicit", "github", "t", "b", dig("seal"), "t0", []string{"l"}, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if op.Generation != 4 || op.CWD != "/w" || op.Holder == nil || op.Holder.SessionID != "s" {
		t.Fatalf("authority not bound: %+v", op)
	}
	if err := ValidateChildOperationAuthority(record, op); err != nil {
		t.Fatalf("bound op rejected by its own record: %v", err)
	}
	// 생성 권한이 없는 generation 0 실행은 holder 없이 봉인할 수 없다.
	unleased := record
	unleased.Execution = covBranchChildExecution(0, "/w")
	if _, _, err := SealChildOperation(unleased, opA, "explicit", "github", "t", "b", dig("seal"), "t0", []string{"l"}, []string{"a"}); err == nil || err.Error() != "invalid unleased child authority" {
		t.Fatalf("unleased holder = %v", err)
	}
	// 다른 generation으로 바뀐 실행은 reconcile을 요구한다.
	moved := record
	moved.Execution = covBranchChildExecution(5, "/w")
	if err := ValidateChildOperationAuthority(moved, op); err == nil || !strings.Contains(err.Error(), "explicit reconcile required") {
		t.Fatalf("moved generation = %v", err)
	}
	moved.Execution = covBranchChildExecution(4, "/elsewhere")
	if err := ValidateChildOperationAuthority(moved, op); err == nil || !strings.Contains(err.Error(), "explicit reconcile required") {
		t.Fatalf("moved root = %v", err)
	}
}

func TestCovBranchChildBeginOperationRejections(t *testing.T) {
	record := model.IssueOpsRecord{IssueURL: childParentURL}
	req := sealedChild(t, record, opA, dig("a"))

	moved := record
	moved.IssueURL = "https://github.com/Org/Repo/issues/9"
	if _, _, started, err := BeginChildOperation(moved, req, ""); err == nil || started || err.Error() != "child operation parent/project changed" {
		t.Fatalf("request authority = %v %v", started, err)
	}

	// 저장된 작업의 authority가 바뀌었으면 재시도 전에 거부한다.
	stored := req
	stored.Status = model.IssueCreateIntentNotInvoked
	stored.Generation = 7
	stored.Holder = &model.NativeActor{Host: "h", SessionID: "s"}
	stored.CWD = "/w"
	record.ChildCreateOperations = []model.ChildCreateOperation{stored}
	if _, _, started, err := BeginChildOperation(record, req, opA); err == nil || started || err.Error() != "child operation authority changed" {
		t.Fatalf("stored authority = %v %v", started, err)
	}

	// 완료되지 않은 다른 작업이 있으면 새 작업을 시작할 수 없다.
	other := sealedChild(t, model.IssueOpsRecord{IssueURL: childParentURL}, opB, dig("b"))
	record.ChildCreateOperations = []model.ChildCreateOperation{other}
	if _, _, started, err := BeginChildOperation(record, req, ""); err == nil || started || err.Error() != "unresolved child operation; reconcile before creating" {
		t.Fatalf("unresolved sibling = %v %v", started, err)
	}
	other.Status = model.IssueCreateIntentCompleted
	other.CanonicalURL = "https://github.com/Org/Repo/issues/3"
	record.ChildCreateOperations = []model.ChildCreateOperation{other}
	got, op, started, err := BeginChildOperation(record, req, "")
	if err != nil || !started || op.OperationID != opA || len(got.ChildCreateOperations) != 2 {
		t.Fatalf("completed sibling must not block: %v %v %+v", started, err, got.ChildCreateOperations)
	}
}

func TestCovBranchChildRecordOutcomeRejections(t *testing.T) {
	record := model.IssueOpsRecord{IssueURL: childParentURL}
	if _, err := RecordChildOutcome(record, opA, model.IssueCreateIntentCompleted, "", "", "t"); err == nil || err.Error() != "child operation not found" {
		t.Fatalf("missing = %v", err)
	}
	if _, err := CompleteChildOperation(record, opA, "https://github.com/Org/Repo/issues/3", "t"); err == nil || err.Error() != "child operation not found" {
		t.Fatalf("complete missing = %v", err)
	}
	op := sealedChild(t, record, opA, dig("a"))
	record.ChildCreateOperations = []model.ChildCreateOperation{op}
	if _, err := RecordChildOutcome(record, opA, model.IssueCreateIntentPending, "", "", "t"); err == nil || !strings.Contains(err.Error(), "illegal issue create intent transition") {
		t.Fatalf("illegal transition = %v", err)
	}
	if _, err := RecordChildOutcome(record, opA, model.IssueCreateIntentCompleted, "https://github.com/Other/Repo/issues/3", "", "t"); err == nil || err.Error() != "child operation project mismatch" {
		t.Fatalf("project mismatch = %v", err)
	}
	if _, err := RecordChildOutcome(record, opA, model.IssueCreateIntentCompleted, childParentURL, "", "t"); err == nil || err.Error() != "child operation canonical URL changed" {
		t.Fatalf("parent URL as child = %v", err)
	}
	observed, err := RecordChildOutcome(record, opA, model.IssueCreateIntentURLObserved, "https://github.com/Org/Repo/issues/3", "", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RecordChildOutcome(observed, opA, model.IssueCreateIntentCompleted, "https://github.com/Org/Repo/issues/4", "", "t2"); err == nil || err.Error() != "child operation canonical URL changed" {
		t.Fatalf("canonical swap = %v", err)
	}
	// 권한이 바뀐 parent에서는 결과 기록 자체를 거부한다.
	moved := observed
	moved.IssueURL = "https://github.com/Org/Repo/issues/9"
	if _, err := RecordChildOutcome(moved, opA, model.IssueCreateIntentCompleted, "", "", "t"); err == nil || err.Error() != "child operation parent/project changed" {
		t.Fatalf("moved parent = %v", err)
	}
}

func TestCovBranchChildStatusTerminalCleanupReceipt(t *testing.T) {
	cases := []struct {
		name string
		ref  model.IssueOpsChildCycleRef
		want bool
	}{
		{"unvalidated", model.IssueOpsChildCycleRef{ValidationVerdict: "accepted", ValidationEvidence: []string{"e"}}, false},
		{"accepted with evidence", model.IssueOpsChildCycleRef{ValidatedAt: "t", ValidationVerdict: "accepted", ValidationEvidence: []string{"e"}}, true},
		{"accepted without evidence", model.IssueOpsChildCycleRef{ValidatedAt: "t", ValidationVerdict: "accepted"}, false},
		{"dropped long reason", model.IssueOpsChildCycleRef{ValidatedAt: "t", ValidationVerdict: "dropped", ValidationReason: "no longer needed"}, true},
		{"dropped short reason", model.IssueOpsChildCycleRef{ValidatedAt: "t", ValidationVerdict: "dropped", ValidationReason: "nope"}, false},
		{"rejected", model.IssueOpsChildCycleRef{ValidatedAt: "t", ValidationVerdict: "rejected", ValidationReason: "long enough reason", ValidationEvidence: []string{"e"}}, false},
	}
	for _, tc := range cases {
		if got := issueOpsChildRefHasTerminalCleanupReceipt(tc.ref); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, got, tc.want)
		}
		// 영수증이 없는 child가 사라지면 orphan으로 보고된다.
		status := BuildChildStatus(model.IssueOpsRecord{ID: "p", ChildCycles: []model.IssueOpsChildCycleRef{withCovCycleID(tc.ref)}}, nil)
		if orphaned := len(status.Orphaned) == 1; orphaned == tc.want {
			t.Errorf("%s: orphaned = %v with receipt %v", tc.name, status.Orphaned, tc.want)
		}
	}
}

func withCovCycleID(ref model.IssueOpsChildCycleRef) model.IssueOpsChildCycleRef {
	ref.CycleID = "c"
	return ref
}

func TestCovBranchChildRetargetRequiresHolderRejectsInvalidExecution(t *testing.T) {
	needs, err := RetargetRequiresHolder(model.IssueOpsRecord{Execution: &model.Execution{}})
	if err == nil || needs || !strings.Contains(err.Error(), "invalid IssueOps execution v1 record") {
		t.Fatalf("needs=%v err=%v", needs, err)
	}
	if errors.Unwrap(err) == nil {
		t.Fatal("validation cause must stay wrapped")
	}
}

func TestCovBranchChildAISlopCleanCompletionMissing(t *testing.T) {
	full := model.IssueOpsRecord{AISlopCleanAt: "t", AISlopCleanHead: "h", AISlopCleanFingerprint: "f", AISlopCleanCategories: []string{"c"}, AISlopCleanVerification: []string{"v"}}
	if got := AISlopCleanCompletionMissing(full); len(got) != 0 {
		t.Fatalf("full = %v", got)
	}
	blank := model.IssueOpsRecord{AISlopCleanAt: " ", AISlopCleanHead: " ", AISlopCleanFingerprint: " ", AISlopCleanCategories: []string{" ", "a\x00b"}, AISlopCleanVerification: []string{""}}
	want := []string{"ai_slop_clean_at", "ai_slop_clean_head", "ai_slop_clean_fingerprint", "cleanup_evidence", "verification_evidence"}
	if got := AISlopCleanCompletionMissing(blank); !slices.Equal(got, want) {
		t.Fatalf("blank = %v", got)
	}
	// 각 필드는 독립적으로 보고된다.
	noFingerprint := full
	noFingerprint.AISlopCleanFingerprint = ""
	if got := AISlopCleanCompletionMissing(noFingerprint); !slices.Equal(got, []string{"ai_slop_clean_fingerprint"}) {
		t.Fatalf("fingerprint only = %v", got)
	}
}

func TestCovBranchChildValidateBranchRejectsMissingNumberOrSlug(t *testing.T) {
	for _, branch := range []string{"-fix", "12", "12-", "x12-fix", "1.2-fix"} {
		if err := ValidateBranch(branch); err == nil || !strings.Contains(err.Error(), "must start with the issue number") {
			t.Errorf("%q = %v", branch, err)
		}
	}
	for _, branch := range []string{"", "  ", "12-fix", "7-a-b"} {
		if err := ValidateBranch(branch); err != nil {
			t.Errorf("%q = %v", branch, err)
		}
	}
}

func TestCovBranchChildChangeTierSurfaces(t *testing.T) {
	cases := []struct {
		paths []string
		want  ChangeTier
	}{
		{[]string{"api/service.proto"}, ChangeTierContract},
		{[]string{"testdata/x.golden.json"}, ChangeTierContract},
		{[]string{"docs/OpenAPI.yaml"}, ChangeTierContract},
		{[]string{"web/swagger.json"}, ChangeTierContract},
		{[]string{"internal/contract/x.go"}, ChangeTierContract},
		{[]string{"README"}, ChangeTierDocsOnly},
		{[]string{"src/Readme.txt", "notes/a.md"}, ChangeTierDocsOnly},
		{[]string{"skills/x/SKILL.txt"}, ChangeTierDocsOnly},
		{[]string{"README", "main.go"}, ChangeTierDefault},
		{[]string{"  ", ""}, ChangeTierDocsOnly},
		{[]string{"internal\\auth\\x.go", "README.md"}, ChangeTierSchemaAuth},
	}
	for _, tc := range cases {
		if got := ClassifyChangeTier(tc.paths); got != tc.want {
			t.Errorf("%v = %v, want %v", tc.paths, got, tc.want)
		}
	}
	if PathIsFrontendChange("  ") || PathIsFrontendChange("") {
		t.Fatal("blank path is not a frontend change")
	}
	if !PathIsFrontendChange("  SRC\\App.TSX ") {
		t.Fatal("normalized tsx path should be frontend")
	}
}

func TestCovBranchChildGrillReadinessMissing(t *testing.T) {
	trivial := model.IssueOpsRecord{Intent: &model.IssueOpsIntentContract{IntentClass: "Trivial"}}
	want := []string{"issue_url", "branch", "split_decision", "domain_review"}
	if got := GrillReadinessMissing(trivial); !slices.Equal(got, want) {
		t.Fatalf("trivial = %v", got)
	}
	nontrivial := model.IssueOpsRecord{IssueURL: "u", Branch: "1-a", Intent: &model.IssueOpsIntentContract{IntentClass: "feature"}, Decisions: []model.IssueOpsDecision{{Kind: "Scope"}}, DomainReview: &model.IssueOpsDomainReview{ReviewedAt: "t"}}
	got := GrillReadinessMissing(nontrivial)
	if !slices.Equal(got, []string{"plan_prep_decisions", "plan_prep_related_issues", "plan_prep_web_research", "plan_prep_codebase_survey"}) {
		t.Fatalf("nontrivial = %v", got)
	}
}
