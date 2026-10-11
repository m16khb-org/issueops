package issueops

import (
	"errors"
	"slices"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestNormalizeMergeMethodDefaultsToSquashAndRejectsUnknownStrategies(t *testing.T) {
	for raw, want := range map[string]string{
		"": model.RemoteMergeMethodSquash, "  ": model.RemoteMergeMethodSquash,
		" Merge ": model.RemoteMergeMethodMerge, "REBASE": model.RemoteMergeMethodRebase, "squash": model.RemoteMergeMethodSquash,
	} {
		got, err := NormalizeMergeMethod(raw)
		if err != nil || got != want {
			t.Fatalf("NormalizeMergeMethod(%q)=%q,%v want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{"fast-forward", "squash merge", "ff"} {
		if got, err := NormalizeMergeMethod(raw); err == nil || got != "" {
			t.Fatalf("NormalizeMergeMethod(%q)=%q,%v", raw, got, err)
		}
	}
}

func covIZDoneRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{
		Phase: model.IssueOpsPhaseDone,
		Execution: &model.Execution{
			Lease:      model.WriteLease{Status: model.LeaseStatusReleased},
			Completion: &model.ExecutionCompletion{FinalHead: " abc123 ", RemoteArtifactURL: "https://github.com/acme/repo/pull/9"},
		},
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Provider: "github", URL: " https://github.com/acme/repo/pull/9 "},
	}
}

func TestResolveMergeTargetAdmitsOnlyCompletedReleasedCyclesWithMatchingArtifact(t *testing.T) {
	target, err := ResolveMergeTarget(covIZDoneRecord())
	if err != nil {
		t.Fatal(err)
	}
	if target != (MergeTarget{Provider: "github", URL: "https://github.com/acme/repo/pull/9", ExpectedHead: "abc123"}) {
		t.Fatalf("target=%+v", target)
	}
	record := covIZDoneRecord()
	record.Execution.Completion.RemoteArtifactURL = ""
	if _, err := ResolveMergeTarget(record); err != nil {
		t.Fatalf("completion without recorded artifact URL must defer to the verified artifact: %v", err)
	}

	for _, tc := range []struct {
		name   string
		mutate func(*model.IssueOpsRecord)
		want   string
	}{
		{"phase", func(r *model.IssueOpsRecord) { r.Phase = model.IssueOpsPhasePR }, "phase done"},
		{"no execution", func(r *model.IssueOpsRecord) { r.Execution = nil }, "recorded execution completion"},
		{"no completion", func(r *model.IssueOpsRecord) { r.Execution.Completion = nil }, "recorded execution completion"},
		{"blank final head", func(r *model.IssueOpsRecord) { r.Execution.Completion.FinalHead = "  " }, "recorded execution completion"},
		{"active lease", func(r *model.IssueOpsRecord) { r.Execution.Lease.Status = model.LeaseStatusActive }, "released execution lease"},
		{"claimable lease", func(r *model.IssueOpsRecord) { r.Execution.Lease.Status = model.LeaseStatusClaimable }, "released execution lease"},
		{"no artifact", func(r *model.IssueOpsRecord) { r.RemoteArtifact = nil }, "verified remote artifact"},
		{"blank artifact url", func(r *model.IssueOpsRecord) { r.RemoteArtifact.URL = " " }, "verified remote artifact"},
		{"blank artifact provider", func(r *model.IssueOpsRecord) { r.RemoteArtifact.Provider = "" }, "verified remote artifact"},
		{"artifact mismatch", func(r *model.IssueOpsRecord) {
			r.Execution.Completion.RemoteArtifactURL = "https://github.com/acme/repo/pull/10"
		}, "does not match the verified remote artifact"},
	} {
		record := covIZDoneRecord()
		tc.mutate(&record)
		if target, err := ResolveMergeTarget(record); err == nil || !strings.Contains(err.Error(), tc.want) || target != (MergeTarget{}) {
			t.Fatalf("%s: target=%+v err=%v", tc.name, target, err)
		}
	}
}

func covIZBlockerCodes(blockers []model.RemoteMergeBlocker) []string {
	out := make([]string, 0, len(blockers))
	for _, b := range blockers {
		out = append(out, b.Code)
	}
	return out
}

func TestMergeBlockersReportsEveryObservedReasonInOrder(t *testing.T) {
	clean := model.RemotePullRequestMergeState{State: "open", HeadOID: " ABC123 ", Checks: model.RemoteChecksPassing}
	if got := MergeBlockers(clean, "abc123"); got == nil || len(got) != 0 {
		t.Fatalf("clean PR has blockers (or nil slice): %#v", got)
	}
	if got := MergeBlockers(model.RemotePullRequestMergeState{State: "open", HeadOID: "abc", Checks: model.RemoteChecksNone}, "abc"); len(got) != 0 {
		t.Fatalf("PR without checks blocked: %v", got)
	}
	merged := model.RemotePullRequestMergeState{State: "merged", HeadOID: "other", Checks: model.RemoteChecksFailing, Conflict: true, Blocked: "review"}
	if got := MergeBlockers(merged, "abc"); got == nil || len(got) != 0 {
		t.Fatalf("merged artifact must have no blockers: %#v", got)
	}

	worst := model.RemotePullRequestMergeState{
		State: "closed", HeadOID: "def456", Checks: model.RemoteChecksFailing, Failing: []string{"lint", "unit"}, Conflict: true, Blocked: " needs review ",
	}
	got := MergeBlockers(worst, "abc123")
	want := []string{model.RemoteMergeBlockerNotOpen, model.RemoteMergeBlockerHeadMismatch, model.RemoteMergeBlockerChecksFailing, model.RemoteMergeBlockerConflict, model.RemoteMergeBlocked}
	if !slices.Equal(covIZBlockerCodes(got), want) {
		t.Fatalf("codes=%v want %v", covIZBlockerCodes(got), want)
	}
	for i, fragment := range []string{"closed", "def456 is not the completed final head abc123", "lint, unit", "conflicts", "needs review"} {
		if !strings.Contains(got[i].Message, fragment) {
			t.Fatalf("blocker %d message %q lacks %q", i, got[i].Message, fragment)
		}
	}

	pending := MergeBlockers(model.RemotePullRequestMergeState{State: "open", HeadOID: "abc", Checks: model.RemoteChecksPending, Pending: []string{"e2e"}}, "abc")
	if len(pending) != 1 || pending[0].Code != model.RemoteMergeBlockerChecksPending || !strings.Contains(pending[0].Message, "e2e") {
		t.Fatalf("pending=%+v", pending)
	}
}

func TestRemoteCompletionPreconditionsNameTheMissingEvidence(t *testing.T) {
	linked := model.IssueOpsRecord{IssueURL: "https://github.com/a/b/issues/1", RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "u"}}
	if err := ValidateCompletionMerge(linked); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCompletionMerge(model.IssueOpsRecord{}); err == nil || !strings.Contains(err.Error(), "merge evidence") {
		t.Fatalf("merge without artifact: %v", err)
	}
	if err := ValidateReflectCompletion(linked); err != nil {
		t.Fatal(err)
	}
	noIssue := linked
	noIssue.IssueURL = "  "
	if err := ValidateReflectCompletion(noIssue); err == nil || !strings.Contains(err.Error(), "linked issue") {
		t.Fatalf("reflect without issue: %v", err)
	}
	noArtifact := linked
	noArtifact.RemoteArtifact = nil
	if err := ValidateReflectCompletion(noArtifact); err == nil || !strings.Contains(err.Error(), "remote artifact") {
		t.Fatalf("reflect without artifact: %v", err)
	}
	if err := ValidateCloseIssue(linked); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCloseIssue(noIssue); err == nil {
		t.Fatal("close without a linked issue accepted")
	}
}

func covIZChildOperation() model.ChildCreateOperation {
	return model.ChildCreateOperation{
		IssueOpsIssueCreateIntent: model.IssueOpsIssueCreateIntent{
			Provider: "github", ProjectAuthority: "github.com/acme/repo", Status: model.IssueCreateIntentPending,
		},
		ParentURL: "https://github.com/acme/repo/issues/1",
	}
}

func TestValidateRecordInvariantsChecksChildOperationsProjectsAndHolders(t *testing.T) {
	validate := func(op model.ChildCreateOperation) error {
		return ValidateRecordInvariants(model.IssueOpsRecord{ChildCreateOperations: []model.ChildCreateOperation{op}})
	}
	if err := validate(covIZChildOperation()); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*model.ChildCreateOperation)
		want   string
	}{
		{"intent invariant", func(op *model.ChildCreateOperation) { op.Status = model.IssueCreateIntentInvokedUnknown }, "requires failure"},
		{"authority differs from parent", func(op *model.ChildCreateOperation) { op.ProjectAuthority = "github.com/other/repo" }, "child operation project mismatch"},
		{"parent not an issue url", func(op *model.ChildCreateOperation) { op.ParentURL = "https://github.com/acme/repo/pull/1" }, "child operation project mismatch"},
		{"unknown provider", func(op *model.ChildCreateOperation) { op.Provider = "bitbucket" }, "child operation project mismatch"},
		{"canonical in other project", func(op *model.ChildCreateOperation) {
			op.Status, op.Failure, op.CanonicalURL = model.IssueCreateIntentURLObserved, "pending receipt", "https://github.com/other/repo/issues/2"
		}, "canonical project mismatch"},
		{"canonical equals parent", func(op *model.ChildCreateOperation) {
			op.Status, op.Failure, op.CanonicalURL = model.IssueCreateIntentURLObserved, "pending receipt", op.ParentURL
		}, "canonical project mismatch"},
		{"holder without process", func(op *model.ChildCreateOperation) {
			op.Holder = &model.NativeActor{Host: "codex", SessionID: "s"}
		}, "PID reuse-safe"},
	} {
		op := covIZChildOperation()
		tc.mutate(&op)
		if err := validate(op); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	observed := covIZChildOperation()
	observed.Status, observed.Failure, observed.CanonicalURL = model.IssueCreateIntentURLObserved, "pending receipt", "https://github.com/acme/repo/issues/2"
	observed.Holder = &model.NativeActor{Host: "codex", SessionID: "s", SessionProcess: &model.NativeProcessReceipt{PID: 1, StartedAt: "t", Executable: "x"}}
	if err := validate(observed); err != nil {
		t.Fatalf("valid observed child with holder rejected: %v", err)
	}
}

func TestValidateRecordInvariantsRejectsInvalidExecutionAndIntentStates(t *testing.T) {
	if err := ValidateRecordInvariants(model.IssueOpsRecord{Execution: &model.Execution{}}); err == nil {
		t.Fatal("empty execution accepted")
	}
	completedWithoutURL := &model.IssueOpsIssueCreateIntent{Status: model.IssueCreateIntentCompleted}
	if err := ValidateRecordInvariants(model.IssueOpsRecord{IssueCreateIntent: completedWithoutURL}); err == nil || !strings.Contains(err.Error(), "canonical_url") {
		t.Fatalf("completed intent without URL: %v", err)
	}
	if err := ValidateRecordInvariants(model.IssueOpsRecord{}); err != nil {
		t.Fatalf("empty record rejected: %v", err)
	}
}

func TestValidateRecordInvariantsNamesTheInvalidPlanPrepItem(t *testing.T) {
	evidence := model.IssueOpsPlanPrepItem{Status: "evidence", Evidence: []string{"adr"}}
	waived := model.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "n/a"}
	valid := model.IssueOpsPlanPrep{PriorDecisions: evidence, RelatedIssues: waived, WebResearch: evidence, CodebaseSurvey: waived}
	if err := ValidateRecordInvariants(model.IssueOpsRecord{PlanPrep: &valid}); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*model.IssueOpsPlanPrep){
		"prior_decisions": func(p *model.IssueOpsPlanPrep) { p.PriorDecisions = model.IssueOpsPlanPrepItem{Status: "evidence"} },
		"related_issues": func(p *model.IssueOpsPlanPrep) {
			p.RelatedIssues = model.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: " "}
		},
		"web_research":    func(p *model.IssueOpsPlanPrep) { p.WebResearch = model.IssueOpsPlanPrepItem{Status: "skipped"} },
		"codebase_survey": func(p *model.IssueOpsPlanPrep) { p.CodebaseSurvey = model.IssueOpsPlanPrepItem{} },
	} {
		prep := valid
		mutate(&prep)
		err := ValidateRecordInvariants(model.IssueOpsRecord{PlanPrep: &prep})
		if err == nil || !strings.Contains(err.Error(), "plan_prep "+name+" ") {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if err := ValidateRecordInvariants(model.IssueOpsRecord{PlanPrep: &model.IssueOpsPlanPrep{
		PriorDecisions: evidence, RelatedIssues: evidence, WebResearch: evidence, CodebaseSurvey: model.IssueOpsPlanPrepItem{Status: "skipped"},
	}}); err == nil || !strings.Contains(err.Error(), `status "skipped" is invalid`) {
		t.Fatalf("unknown status: %v", err)
	}
}

func TestResolveRecordProviderPrefersBranchThenArtifactThenURL(t *testing.T) {
	for _, tc := range []struct {
		name   string
		record model.IssueOpsRecord
		want   string
	}{
		{"branch prepare wins", model.IssueOpsRecord{
			BranchPrepare: &model.IssueOpsBranchPrepare{Provider: "gitlab"}, RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Provider: "github"}, IssueURL: "https://github.com/a/b/issues/1",
		}, "gitlab"},
		{"artifact when branch provider empty", model.IssueOpsRecord{
			BranchPrepare: &model.IssueOpsBranchPrepare{}, RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Provider: "gitlab"}, IssueURL: "https://github.com/a/b/issues/1",
		}, "gitlab"},
		{"github url", model.IssueOpsRecord{RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{}, IssueURL: "https://github.com/a/b/issues/1"}, "github"},
		{"gitlab url", model.IssueOpsRecord{IssueURL: "https://gitlab.corp.example/a/b/-/issues/1"}, "gitlab"},
		{"unknown host needs explicit provider", model.IssueOpsRecord{IssueURL: "https://code.corp.example/a/b/issues/1"}, ""},
	} {
		if got := ResolveRecordProvider(tc.record); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestReviewReflectionStampAndRounds(t *testing.T) {
	if err := ValidateReviewReflectionStamp(model.IssueOpsRecord{}); err == nil {
		t.Fatal("stamp without a review accepted")
	}
	if err := ValidateReviewReflectionStamp(model.IssueOpsRecord{DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{}}); err != nil {
		t.Fatal(err)
	}
	review := &model.IssueOpsDevilsAdvocateReview{
		Verdict: "pass", Findings: nil,
		History: []model.IssueOpsDevilsAdvocateRound{{Verdict: "revise", Findings: []string{"a", "b", "c"}}, {Verdict: "revise", Findings: []string{"d"}}},
	}
	want := []model.PlanReviewRound{{Verdict: "revise", Findings: 3}, {Verdict: "revise", Findings: 1}, {Verdict: "pass", Findings: 0}}
	if got := PlanReviewRounds(review); !slices.Equal(got, want) {
		t.Fatalf("rounds=%+v want=%+v", got, want)
	}
	if got := PlanReviewRounds(&model.IssueOpsDevilsAdvocateReview{Verdict: "stop", Findings: []string{"x"}}); !slices.Equal(got, []model.PlanReviewRound{{Verdict: "stop", Findings: 1}}) {
		t.Fatalf("single round=%+v", got)
	}
}

func TestRequireSealedArtifactDirRejectsMissingDirectory(t *testing.T) {
	for name, record := range map[string]model.IssueOpsRecord{
		"no execution": {},
		"blank dir":    {Execution: &model.Execution{Workspace: model.Workspace{ArtifactDir: "  "}}},
	} {
		if err := RequireSealedArtifactDir(record); !errors.Is(err, ErrSealedArtifactDirMissing) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if err := RequireSealedArtifactDir(model.IssueOpsRecord{Execution: &model.Execution{Workspace: model.Workspace{ArtifactDir: ".issueops/x"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestStartCycleRequestAndRecordInitialState(t *testing.T) {
	if err := ValidateNewCycleRequest(model.IssueOpsStartRequest{New: true, Branch: " feat/x "}); err == nil || !strings.Contains(err.Error(), "branchless") {
		t.Fatalf("new cycle with branch: %v", err)
	}
	for _, req := range []model.IssueOpsStartRequest{{New: true}, {New: true, Branch: "  "}, {Branch: "feat/x"}, {}} {
		if err := ValidateNewCycleRequest(req); err != nil {
			t.Fatalf("%+v rejected: %v", req, err)
		}
	}
	record := NewCycleRecord("io-1", "/repo", "feat/x", "now")
	if !record.OK || record.SchemaVersion != model.IssueOpsSchemaVersion || record.ID != "io-1" || record.Repo != "/repo" || record.Branch != "feat/x" ||
		record.Phase != model.IssueOpsPhaseProblem || record.CreatedAt != "now" || record.UpdatedAt != "now" || record.Feedback == nil || len(record.Feedback) != 0 {
		t.Fatalf("record=%+v", record)
	}
	if record.Execution != nil || record.IssueURL != "" {
		t.Fatalf("new cycle carries state: %+v", record)
	}
}

func TestRenderTrackedPlanReviewListsEveryRoundAndMasksHarnessValues(t *testing.T) {
	digest := strings.Repeat("ab", 32)
	review := &model.IssueOpsDevilsAdvocateReview{
		Verdict: "pass",
		History: []model.IssueOpsDevilsAdvocateRound{
			{Verdict: "revise", Findings: []string{" 해시 " + digest + " 불일치 ", "  ", "/Users/dev/work/plan.md 경로 노출"}},
			{Verdict: "custom", Waived: true, WaiverRationale: " 사유 /home/ci/x "},
		},
	}
	got := RenderTrackedPlanReview(review)
	want := "# 계획 검토 기록\n" +
		"\n## 1차: 수정 요청\n\n- 해시 [해시 생략] 불일치\n- [로컬 경로 생략] 경로 노출\n" +
		"\n## 2차: custom\n\n생략: 사유 [로컬 경로 생략]\n" +
		"\n## 3차: 통과\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
	if len(review.History) != 2 {
		t.Fatal("render mutated the review history")
	}
	if strings.Contains(got, digest) || strings.Contains(got, "/Users/dev") {
		t.Fatal("harness values leaked")
	}
}

func TestRenderSectionRejectsUnknownKindBeforeRendering(t *testing.T) {
	if _, _, _, err := RenderSection(SectionInput{Section: "release-notes"}, 100); err == nil || !strings.Contains(err.Error(), "unsupported issue body section") {
		t.Fatalf("err=%v", err)
	}
	if _, _, _, err := RenderSection(SectionInput{Section: model.IssueBodySectionCompletion}, 100); err == nil || !strings.Contains(err.Error(), "completion payload is required") {
		t.Fatalf("err=%v", err)
	}
}

func TestRenderDevilsAdvocateSectionFallsBackToCurrentVerdictWhenNoRounds(t *testing.T) {
	section := RenderDevilsAdvocateSection(SectionInput{Verdict: "stop", Findings: []string{" 범위 불명 ", "", "/Users/me/x 에서"}})
	if !strings.Contains(section, "계획 검토: 1차 중단(지적 3건)\n") {
		t.Fatalf("flow line missing: %s", section)
	}
	if !strings.Contains(section, "중단 이유:\n- 범위 불명\n- [로컬 경로 생략] 에서\n"+devilsAdvocateEndMarker) {
		t.Fatalf("stop reasons: %s", section)
	}
	revise := RenderDevilsAdvocateSection(SectionInput{Verdict: "revise", Findings: []string{"hidden finding"}})
	if strings.Contains(revise, "hidden finding") || !strings.Contains(revise, "1차 수정 요청(지적 1건)") {
		t.Fatalf("non-stop verdict leaks findings or loses counts: %s", revise)
	}
}

func TestPlanLinkValidationsExplainEachRefusal(t *testing.T) {
	if err := ValidateWorkspaceLinkPath("plan_path", ""); err == nil || !strings.Contains(err.Error(), "plan_path is required") {
		t.Fatalf("empty: %v", err)
	}
	for _, path := range []string{"a/../b", "..", "a\x00b"} {
		if err := ValidateWorkspaceLinkPath("worktree_path", path); err == nil || !strings.Contains(err.Error(), "worktree_path must not contain path traversal") {
			t.Fatalf("%q: %v", path, err)
		}
	}
	if err := ValidateWorkspaceLinkPath("plan_path", "/ok/plan.md"); err != nil {
		t.Fatal(err)
	}

	ready := model.IssueOpsRecord{IssueURL: "https://github.com/a/b/issues/1", Branch: "b", BranchPrepare: &model.IssueOpsBranchPrepare{LinkVerified: true}, WorktreePath: "/wt"}
	if err := ValidatePlanLinkReadiness(ready, nil); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLinkBranchEvidence(model.IssueOpsRecord{}, "plan"); err == nil || !strings.Contains(err.Error(), "cannot link plan before branch evidence: missing issue_url, branch, branch_prepare") {
		t.Fatalf("evidence: %v", err)
	}
	unverified := ready
	unverified.BranchPrepare = &model.IssueOpsBranchPrepare{}
	if err := ValidatePlanLinkReadiness(unverified, nil); err == nil || !strings.Contains(err.Error(), "branch_link_verified") {
		t.Fatalf("unverified link: %v", err)
	}
	noTree := ready
	noTree.WorktreePath = " "
	if err := ValidatePlanLinkReadiness(noTree, nil); err == nil || !strings.Contains(err.Error(), "linked worktree") {
		t.Fatalf("worktree: %v", err)
	}
	if err := ValidatePlanLinkReadiness(ready, []string{"design_review", "approval"}); err == nil || !strings.Contains(err.Error(), "missing design_review, approval") {
		t.Fatalf("design: %v", err)
	}

	if err := ValidatePlanLinkLocation("/wt/plan.md", "/wt", false, true); err == nil || !strings.Contains(err.Error(), "does not exist: /wt/plan.md") {
		t.Fatalf("missing plan: %v", err)
	}
	if err := ValidatePlanLinkLocation("/x/plan.md", "/wt", true, false); err == nil || !strings.Contains(err.Error(), "inside linked worktree: /wt") {
		t.Fatalf("outside plan: %v", err)
	}
	if err := ValidatePlanLinkLocation("/wt/plan.md", "/wt", true, true); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLinkedPlanContainment("/wt", true); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLinkedPlanSections(nil); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLinkedPlanSections([]string{"## 검증", "## 범위"}); err == nil || !strings.Contains(err.Error(), "## 검증, ## 범위") {
		t.Fatalf("sections: %v", err)
	}
}

func TestPlanLinkIsUnchangedOnlyAllowsIdempotentRelink(t *testing.T) {
	if unchanged, err := PlanLinkIsUnchanged(" ", false); unchanged || err != nil {
		t.Fatalf("first link: %t %v", unchanged, err)
	}
	if unchanged, err := PlanLinkIsUnchanged("/wt/plan.md", true); !unchanged || err != nil {
		t.Fatalf("same path: %t %v", unchanged, err)
	}
	if unchanged, err := PlanLinkIsUnchanged("/wt/plan.md", false); unchanged || err == nil || !strings.Contains(err.Error(), "edit the linked plan in place") {
		t.Fatalf("replacement: %t %v", unchanged, err)
	}
}

func TestApplyPlanAndWorktreeLinkTouchOnlyTheirField(t *testing.T) {
	original := model.IssueOpsRecord{PlanPath: "old", WorktreePath: "wt", UpdatedAt: "before"}
	plan := ApplyPlanLink(original, "new-plan", "now")
	if plan.PlanPath != "new-plan" || plan.WorktreePath != "wt" || plan.UpdatedAt != "now" {
		t.Fatalf("plan=%+v", plan)
	}
	tree := ApplyWorktreeLink(original, "new-wt", "now")
	if tree.WorktreePath != "new-wt" || tree.PlanPath != "old" || tree.UpdatedAt != "now" {
		t.Fatalf("tree=%+v", tree)
	}
	if original.PlanPath != "old" || original.WorktreePath != "wt" || original.UpdatedAt != "before" {
		t.Fatal("input mutated")
	}
}

func TestValidateWorktreeDirectoryAndLocation(t *testing.T) {
	if err := ValidateWorktreeDirectory("/wt", true); err != nil {
		t.Fatal(err)
	}
	if err := ValidateWorktreeDirectory("/wt", false); err == nil || !strings.Contains(err.Error(), "/wt") {
		t.Fatalf("non-directory: %v", err)
	}
	good := WorktreeLocation{
		Repo: "/r", Path: "/r.worktrees/x", Parent: "/r.worktrees", ResolvedRepo: "/r", ResolvedPath: "/r.worktrees/x", ResolvedParent: "/r.worktrees",
		InsideParent: true, Exists: true, RepoResolved: true, PathResolved: true, InsideResolvedParent: true,
	}
	if err := ValidateWorktreeLocation(good); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*WorktreeLocation)
		want   string
	}{
		{"empty repo", func(l *WorktreeLocation) { l.Repo = "" }, "absolute or resolvable"},
		{"empty path", func(l *WorktreeLocation) { l.Path = "" }, "absolute or resolvable"},
		{"path is repo", func(l *WorktreeLocation) { l.Path = l.Repo }, "isolated from the source checkout"},
		{"outside parent", func(l *WorktreeLocation) { l.InsideParent = false }, "under sibling worktree directory: /r.worktrees"},
		{"missing", func(l *WorktreeLocation) { l.Exists = false }, "does not exist"},
		{"symlink", func(l *WorktreeLocation) { l.Symlink = true }, "must not be a symlink"},
		{"repo unresolved", func(l *WorktreeLocation) { l.RepoResolved = false }, "source checkout path cannot be resolved"},
		{"path unresolved", func(l *WorktreeLocation) { l.PathResolved = false }, "worktree_path cannot be resolved"},
		{"resolves to repo", func(l *WorktreeLocation) { l.ResolvedPath = l.ResolvedRepo }, "isolated from the source checkout"},
		{"resolves outside parent", func(l *WorktreeLocation) { l.InsideResolvedParent = false }, "must resolve under sibling worktree directory: /r.worktrees"},
	} {
		location := good
		tc.mutate(&location)
		if err := ValidateWorktreeLocation(location); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
}

func TestValidateLinkedWorktreeBranchComparesTrimmedNames(t *testing.T) {
	if err := ValidateLinkedWorktreeBranch(" ", "anything"); err != nil {
		t.Fatalf("no expected branch must accept any: %v", err)
	}
	if err := ValidateLinkedWorktreeBranch(" feat/x ", "feat/x\n"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLinkedWorktreeBranch("feat/x", " "); err == nil || !strings.Contains(err.Error(), "must be a git worktree on IssueOps branch feat/x") {
		t.Fatalf("detached: %v", err)
	}
	if err := ValidateLinkedWorktreeBranch("feat/x", "feat/y"); err == nil || !strings.Contains(err.Error(), "feat/y does not match IssueOps branch feat/x") {
		t.Fatalf("mismatch: %v", err)
	}
}

func TestNormalizeNativeActorRequiresSessionInLocalAncestry(t *testing.T) {
	process := model.NativeProcessReceipt{PID: 42, StartedAt: "t0", Executable: "/bin/codex"}
	input := model.NativeActor{
		Host: " Codex ", SessionID: " s1 ", AgentID: " a1 ",
		SessionProcess:  &model.NativeProcessReceipt{PID: 42, StartedAt: " t0 ", Executable: " /bin/codex "},
		ProcessAncestry: []model.NativeProcessReceipt{{PID: 1, StartedAt: "t", Executable: "init"}, process},
	}
	got, err := NormalizeNativeActor(input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "codex" || got.SessionID != "s1" || got.AgentID != "a1" || *got.SessionProcess != process {
		t.Fatalf("not normalized: %+v", got)
	}
	if input.Host != " Codex " || input.SessionProcess.StartedAt != " t0 " {
		t.Fatal("normalization mutated the caller's actor or receipt")
	}
	got.ProcessAncestry[0].PID = 999
	if input.ProcessAncestry[0].PID != 1 {
		t.Fatal("ancestry slice is shared with the input")
	}

	foreign := input
	foreign.ProcessAncestry = []model.NativeProcessReceipt{{PID: 42, StartedAt: "t0", Executable: "/bin/other"}}
	if _, err := NormalizeNativeActor(foreign); err == nil || !strings.Contains(err.Error(), "not in the local process ancestry") {
		t.Fatalf("PID-reused ancestry accepted: %v", err)
	}
	noAncestry := input
	noAncestry.ProcessAncestry = nil
	if _, err := NormalizeNativeActor(noAncestry); err == nil {
		t.Fatal("actor without ancestry accepted")
	}
	badHost := input
	badHost.Host = "vim"
	if _, err := NormalizeNativeActor(badHost); err == nil || !strings.Contains(err.Error(), "host must be") {
		t.Fatalf("unknown host accepted: %v", err)
	}
	noProcess := input
	noProcess.SessionProcess = nil
	if _, err := NormalizeNativeActor(noProcess); err == nil || !strings.Contains(err.Error(), "session_process receipt") {
		t.Fatalf("missing process receipt: %v", err)
	}
}

func TestValidateObservedNativeProcessRequiresLiveMatchingIdentity(t *testing.T) {
	receipt := model.NativeProcessReceipt{PID: 7, StartedAt: "t0", Executable: "/bin/codex"}
	if err := ValidateObservedNativeProcess(receipt, "live", receipt); err != nil {
		t.Fatal(err)
	}
	if err := ValidateObservedNativeProcess(receipt, "dead", receipt); err == nil || !strings.Contains(err.Error(), "not live: pid=7 status=dead") {
		t.Fatalf("dead: %v", err)
	}
	for name, observed := range map[string]model.NativeProcessReceipt{
		"started":    {PID: 7, StartedAt: "t1", Executable: "/bin/codex"},
		"executable": {PID: 7, StartedAt: "t0", Executable: "/bin/vim"},
	} {
		if err := ValidateObservedNativeProcess(receipt, "live", observed); err == nil || !strings.Contains(err.Error(), "does not match live PID 7") {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestValidateLinkedBranchCleanupApplyRequiresConfirmAndFreshFingerprint(t *testing.T) {
	code, err := ValidateLinkedBranchCleanupApply(model.CleanupLinkedBranchRequest{Fingerprint: "fp"}, "fp")
	if code != "confirm" || err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("unconfirmed: %q %v", code, err)
	}
	code, err = ValidateLinkedBranchCleanupApply(model.CleanupLinkedBranchRequest{Confirm: true, Fingerprint: "old"}, "fp")
	if code != "stale_fingerprint" || err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale: %q %v", code, err)
	}
	if code, err = ValidateLinkedBranchCleanupApply(model.CleanupLinkedBranchRequest{Confirm: true, Fingerprint: "fp"}, "fp"); code != "" || err != nil {
		t.Fatalf("valid: %q %v", code, err)
	}
}

func TestPlanPrepMissingReportsDecisionsAndRelatedIssuesIndependently(t *testing.T) {
	ok := model.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "n/a"}
	prep := &model.IssueOpsPlanPrep{PriorDecisions: model.IssueOpsPlanPrepItem{Status: "evidence"}, RelatedIssues: model.IssueOpsPlanPrepItem{}, WebResearch: ok, CodebaseSurvey: ok}
	if got := PlanPrepMissing(prep); !slices.Equal(got, []string{"plan_prep_decisions", "plan_prep_related_issues"}) {
		t.Fatalf("missing=%v", got)
	}
	prep.PriorDecisions = ok
	if got := PlanPrepMissing(prep); !slices.Equal(got, []string{"plan_prep_related_issues"}) {
		t.Fatalf("missing=%v", got)
	}
}

func TestTargetBranchMatchIgnoresBlankPreparedBase(t *testing.T) {
	record := model.IssueOpsRecord{
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{TargetBranch: "anything"},
		BranchPrepare:  &model.IssueOpsBranchPrepare{BaseBranch: "  "},
	}
	if got := TargetBranchMatchMissing(record); len(got) != 0 {
		t.Fatalf("blank base must not demand a match: %v", got)
	}
	record.BranchPrepare.BaseBranch = "main"
	if got := TargetBranchMatchMissing(record); !slices.Equal(got, []string{"target_branch_match"}) {
		t.Fatalf("mismatch=%v", got)
	}
}

func TestPhaseLedgerStampHandlesNilLedgerAndStaleOnlyNotes(t *testing.T) {
	phase := model.IssueOpsPhasePlan
	stale := MarkLedgerStale(nil, "stale: regression", phase)
	if got := stale[phase]; got.Phase != phase || !slices.Equal(got.Notes, []string{"stale: regression"}) || got.CompletedAt != "" {
		t.Fatalf("nil ledger stale entry=%+v", got)
	}
	stamped := StampForwardTransition(stale, phase, model.IssueOpsPhaseImplement, "now", []string{"plan.md"})
	if got := stamped[phase]; got.Notes != nil || got.CompletedAt != "now" || !slices.Equal(got.Artifacts, []string{"plan.md"}) {
		t.Fatalf("stale-only notes must be cleared to nil: %+v", got)
	}
}

func TestSupersedingArtifactRejectsNonCanonicalOrLetterNumberedURLs(t *testing.T) {
	merged := func(url, body string) ArtifactObservation {
		return ArtifactObservation{URL: url, Merged: true, State: "merged", Body: body}
	}
	if err := ValidateSupersedingArtifact(ArtifactObservation{URL: "https://github.com/acme"}, merged("https://github.com/acme/repo/pull/2", "supersedes #1")); err == nil || !strings.Contains(err.Error(), "original artifact URL") {
		t.Fatalf("project-less original: %v", err)
	}
	if err := ValidateSupersedingArtifact(ArtifactObservation{URL: "https://github.com/acme/repo/pull/1"}, merged("https://github.com/acme", "supersedes #1")); err == nil || !strings.Contains(err.Error(), "superseding artifact URL") {
		t.Fatalf("project-less replacement: %v", err)
	}
	original := ArtifactObservation{URL: "https://github.com/acme/repo/pull/abc"}
	if err := ValidateSupersedingArtifact(original, merged("https://github.com/acme/repo/pull/2", "supersedes abc")); err == nil || !strings.Contains(err.Error(), "does not declare") {
		t.Fatalf("lettered number must not produce a short reference: %v", err)
	}
	if err := ValidateSupersedingArtifact(original, merged("https://github.com/acme/repo/pull/2", "replaces "+original.URL)); err != nil {
		t.Fatalf("full URL reference rejected: %v", err)
	}
}

func TestPlanReviewVerdictLabelKeepsUnknownVerdicts(t *testing.T) {
	for verdict, want := range map[string]string{"pass": "통과", "revise": "수정 요청", "stop": "중단", "needs-info": "needs-info", "": ""} {
		if got := planReviewVerdictLabel(verdict); got != want {
			t.Fatalf("label(%q)=%q want %q", verdict, got, want)
		}
	}
}
