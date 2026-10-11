package issueopspreparation

import (
	"errors"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestDenialExposesCauseAndStructuredFields(t *testing.T) {
	cause := errors.New("probe failed")
	denial := &Denial{Reason: DenialOrcaUnavailable, Code: "orca_down", Cause: cause}
	if denial.Error() != "probe failed" || !errors.Is(denial, cause) {
		t.Fatalf("denial must surface its cause: %q", denial.Error())
	}
	fields := denial.IssueOpsErrorFields()
	if fields["code"] != "orca_unavailable" || fields["probe_code"] != "orca_down" {
		t.Fatalf("fields=%v", fields)
	}
	bare := (&Denial{Reason: DenialInvalidMode, Cause: cause}).IssueOpsErrorFields()
	if _, ok := bare["probe_code"]; ok || bare["code"] != "invalid_mode" {
		t.Fatalf("empty probe code must be omitted: %v", bare)
	}
}

func TestNormalizeDirectReasonRejectsUnsafeValues(t *testing.T) {
	for _, tt := range []struct{ name, reason, want string }{
		{"blank", " \t ", "requires a reason"},
		{"too long", strings.Repeat("a", 513), "at most 512 bytes"},
		{"invalid utf8", "bad\xff", "valid UTF-8"},
		{"newline", "line1\nline2", "control bytes"},
		{"del", "a\x7fb", "control bytes"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NormalizeDirectReason(tt.reason); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err=%v want %q", err, tt.want)
			}
		})
	}
	got, err := NormalizeDirectReason(strings.Repeat("a", 512))
	if err != nil || len(got) != 512 {
		t.Fatalf("512-byte boundary must be accepted: %v", err)
	}
}

func TestValidateIntentRecordChecksCodecThenAuthority(t *testing.T) {
	record, intent := resumeReceiptFixture(t)
	if err := ValidateIntentRecord(record, intent); err != nil {
		t.Fatalf("sealed resume intent rejected: %v", err)
	}
	stale := record
	stale.Execution = &leasecontract.Execution{}
	*stale.Execution = *record.Execution
	stale.Execution.Lease.Generation++
	if err := ValidateIntentRecord(stale, intent); err == nil || !strings.Contains(err.Error(), "authority changed") {
		t.Fatalf("stale generation error=%v", err)
	}
	broken := intent
	broken.Marker = ""
	if err := ValidateIntentRecord(record, broken); err == nil || strings.Contains(err.Error(), "authority changed") {
		t.Fatalf("codec must reject before authority: %v", err)
	}
}

func TestValidateIntentRecordAuthorityPurposeBranches(t *testing.T) {
	record, intent := resumeReceiptFixture(t)

	prepare := intent
	prepare.Purpose = preparationcontract.PurposePrepare
	if err := ValidateIntentRecordAuthority(record, prepare); err == nil || !strings.Contains(err.Error(), "prepare intent authority changed") {
		t.Fatalf("prepare against claimable lease with binding: %v", err)
	}

	unknown := intent
	unknown.Purpose = "other"
	if err := ValidateIntentRecordAuthority(record, unknown); err == nil || !strings.Contains(err.Error(), "unsupported Orca intent purpose") {
		t.Fatalf("unknown purpose: %v", err)
	}

	noLease := intent
	noLease.ResumeLease = nil
	if err := ValidateIntentRecordAuthority(record, noLease); err == nil || !strings.Contains(err.Error(), "resume intent authority changed") {
		t.Fatalf("missing resume lease: %v", err)
	}

	changedLease := intent
	lease := *intent.ResumeLease
	lease.Status = "other"
	changedLease.ResumeLease = &lease
	if err := ValidateIntentRecordAuthority(record, changedLease); err == nil || !strings.Contains(err.Error(), "resume intent authority changed") {
		t.Fatalf("lease drift: %v", err)
	}

	changedBinding := intent
	prior := *intent.PriorBinding
	prior.DispatchID = "other"
	changedBinding.PriorBinding = &prior
	if err := ValidateIntentRecordAuthority(record, changedBinding); err == nil || !strings.Contains(err.Error(), "resume intent authority changed") {
		t.Fatalf("binding drift: %v", err)
	}
}

func TestBindingsEqualTreatsNilAsEqualOnlyToNil(t *testing.T) {
	if !bindingsEqual(nil, nil) {
		t.Fatal("nil bindings must match")
	}
	if bindingsEqual(&preparationcontract.OrcaBinding{}, nil) || bindingsEqual(nil, &preparationcontract.ResumeBinding{}) {
		t.Fatal("nil must not match a present binding")
	}
	left := &preparationcontract.OrcaBinding{RuntimeID: "r", RunID: "run", TerminalPTYID: "pty"}
	right := &preparationcontract.ResumeBinding{RuntimeID: "r", RunID: "run", TerminalPTYID: "pty"}
	if !bindingsEqual(left, right) {
		t.Fatal("identical bindings must match")
	}
	right.TerminalPTYID = "other"
	if bindingsEqual(left, right) {
		t.Fatal("terminal drift must not match")
	}
}

func TestValidateIntentRecordIdentityRequiresEveryPathProof(t *testing.T) {
	record := preparationcontract.Record{
		ID: "io-1", IssueURL: "https://github.com/example/repo/issues/193",
		BranchPrepare: []byte(`{"provider":"github","issue_url":"https://github.com/example/repo/issues/193","link_verified":true}`),
		Execution: &leasecontract.Execution{Mode: "orca", Workspace: leasecontract.Workspace{
			Branch: "work", BaseHead: "head", Driver: "orca",
		}},
	}
	intent := preparationcontract.Intent{
		LifecycleID: "io-1", Probe: preparationcontract.ProbeRequest{Provider: "github", Issue: 193},
		Workspace: preparationcontract.WorkspaceRequest{Branch: "work", BaseHead: "head"},
	}
	all := IntentIdentityPaths{Source: true, Root: true, Parent: true, PreparedRecordedRoot: true, PreparedRoot: true, PreparedParent: true}
	if err := ValidateIntentRecordIdentity(record, intent, all); err != nil {
		t.Fatalf("matching identity rejected: %v", err)
	}

	for name, paths := range map[string]IntentIdentityPaths{
		"source":  {Root: true, Parent: true},
		"root":    {Source: true, Parent: true},
		"parent":  {Source: true, Root: true},
		"nothing": {},
	} {
		if err := ValidateIntentRecordIdentity(record, intent, paths); err == nil || !strings.Contains(err.Error(), "record identity changed") {
			t.Fatalf("%s path proof missing: %v", name, err)
		}
	}

	branch := intent
	branch.Workspace.Branch = "other"
	if err := ValidateIntentRecordIdentity(record, branch, all); err == nil {
		t.Fatal("branch drift accepted")
	}
	head := intent
	head.Workspace.BaseHead = "other"
	if err := ValidateIntentRecordIdentity(record, head, all); err == nil {
		t.Fatal("base head drift accepted")
	}
	direct := record
	direct.Execution = &leasecontract.Execution{}
	*direct.Execution = *record.Execution
	direct.Execution.Workspace.Driver = "git"
	if err := ValidateIntentRecordIdentity(direct, intent, all); err == nil {
		t.Fatal("non-orca workspace driver accepted")
	}
	noExecution := record
	noExecution.Execution = nil
	if err := ValidateIntentRecordIdentity(noExecution, intent, all); err == nil {
		t.Fatal("missing execution accepted")
	}

	prepared := intent
	prepared.Prepared = &preparationcontract.OrcaWorkspaceReceipt{Workspace: preparationcontract.WorkspaceReceipt{Branch: "work", BaseHead: "head"}}
	if err := ValidateIntentRecordIdentity(record, prepared, all); err != nil {
		t.Fatalf("matching prepared identity rejected: %v", err)
	}
	for name, paths := range map[string]IntentIdentityPaths{
		"recorded root": {Source: true, Root: true, Parent: true, PreparedRoot: true, PreparedParent: true},
		"prepared root": {Source: true, Root: true, Parent: true, PreparedRecordedRoot: true, PreparedParent: true},
		"parent":        {Source: true, Root: true, Parent: true, PreparedRecordedRoot: true, PreparedRoot: true},
	} {
		if err := ValidateIntentRecordIdentity(record, prepared, paths); err == nil || !strings.Contains(err.Error(), "prepared workspace identity changed") {
			t.Fatalf("prepared %s proof missing: %v", name, err)
		}
	}
	prepared.Prepared.Workspace.BaseHead = "other"
	if err := ValidateIntentRecordIdentity(record, prepared, all); err == nil || !strings.Contains(err.Error(), "prepared workspace identity changed") {
		t.Fatalf("prepared base head drift: %v", err)
	}

	issue := intent
	issue.Probe.Issue = 194
	if err := ValidateIntentRecordIdentity(record, issue, all); err == nil || !strings.Contains(err.Error(), "intent_identity_mismatch") {
		t.Fatalf("issue drift must surface typed error: %v", err)
	}
	unverified := record
	unverified.BranchPrepare = nil
	if err := ValidateIntentIssueIdentity(unverified, intent); err == nil {
		t.Fatal("missing branch evidence accepted")
	}
	other := intent
	other.LifecycleID = "io-2"
	if err := ValidateIntentIssueIdentity(record, other); err == nil {
		t.Fatal("lifecycle mismatch accepted")
	}
}

func TestValidateLaunchRequestBindsSealedLaunchIdentity(t *testing.T) {
	sealed := func() *preparationcontract.LaunchIdentity {
		return &preparationcontract.LaunchIdentity{PromptPath: "/p", PromptSHA256: "ps", ContextPacketPath: "/c", ContextPacketSHA256: "cs"}
	}
	requested := func() *preparationcontract.LaunchRequest {
		return &preparationcontract.LaunchRequest{PromptPath: "/p", PromptSHA256: "ps", ContextPacketPath: "/c", ContextPacketSHA256: "cs"}
	}
	intent := preparationcontract.Intent{Stage: preparationcontract.IntentStageTerminal, Marker: "m", Launch: sealed()}
	request := preparationcontract.IntentRequest{Stage: intent.Stage, Marker: "m", Launch: requested()}
	if err := ValidateLaunchRequest(request, intent); err != nil {
		t.Fatalf("identical launch rejected: %v", err)
	}
	for name, mutate := range map[string]func(*preparationcontract.IntentRequest, *preparationcontract.Intent){
		"stage": func(r *preparationcontract.IntentRequest, _ *preparationcontract.Intent) {
			r.Stage = preparationcontract.IntentStageRun
		},
		"marker":      func(r *preparationcontract.IntentRequest, _ *preparationcontract.Intent) { r.Marker = "x" },
		"request nil": func(r *preparationcontract.IntentRequest, _ *preparationcontract.Intent) { r.Launch = nil },
		"intent nil":  func(_ *preparationcontract.IntentRequest, i *preparationcontract.Intent) { i.Launch = nil },
		"prompt path": func(r *preparationcontract.IntentRequest, _ *preparationcontract.Intent) { r.Launch.PromptPath = "/x" },
		"prompt sha":  func(r *preparationcontract.IntentRequest, _ *preparationcontract.Intent) { r.Launch.PromptSHA256 = "x" },
		"context path": func(r *preparationcontract.IntentRequest, _ *preparationcontract.Intent) {
			r.Launch.ContextPacketPath = "/x"
		},
		"context sha": func(r *preparationcontract.IntentRequest, _ *preparationcontract.Intent) {
			r.Launch.ContextPacketSHA256 = "x"
		},
	} {
		r, i := request, intent
		r.Launch, i.Launch = requested(), sealed()
		mutate(&r, &i)
		if err := ValidateLaunchRequest(r, i); err == nil || !strings.Contains(err.Error(), "launch identity changed") {
			t.Fatalf("%s drift accepted: %v", name, err)
		}
	}
}

func TestProviderIssueURLAndIdentityRejections(t *testing.T) {
	gitlab := func(url string) *preparationcontract.IssueLinkEvidence {
		return &preparationcontract.IssueLinkEvidence{Provider: "GitLab", IssueURL: url, LinkVerified: true}
	}
	for _, tt := range []struct {
		name     string
		recorded string
		evidence *preparationcontract.IssueLinkEvidence
	}{
		{"nil evidence", "https://github.com/a/b/issues/1", nil},
		{"unknown provider", "https://x.test/a/issues/1", &preparationcontract.IssueLinkEvidence{Provider: "bitbucket", IssueURL: "https://x.test/a/issues/1", LinkVerified: true}},
		{"url differs", "https://github.com/a/b/issues/2", &preparationcontract.IssueLinkEvidence{Provider: "github", IssueURL: "https://github.com/a/b/issues/1", LinkVerified: true}},
		{"github on other host", "https://example.com/a/b/issues/1", &preparationcontract.IssueLinkEvidence{Provider: "github", IssueURL: "https://example.com/a/b/issues/1", LinkVerified: true}},
		{"gitlab url without gitlab marker", "https://example.com/a/b/issues/1", gitlab("https://example.com/a/b/issues/1")},
		{"no host", "/a/-/issues/1", gitlab("/a/-/issues/1")},
		{"path too short", "https://gitlab.com/1", gitlab("https://gitlab.com/1")},
		{"wrong segment", "https://gitlab.com/a/-/merge_requests/1", gitlab("https://gitlab.com/a/-/merge_requests/1")},
		{"zero number", "https://gitlab.com/a/-/issues/0", gitlab("https://gitlab.com/a/-/issues/0")},
		{"non numeric", "https://gitlab.com/a/-/issues/abc", gitlab("https://gitlab.com/a/-/issues/abc")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := PrepareIssueIdentity(tt.recorded, tt.evidence); err == nil || !strings.Contains(err.Error(), "intent_identity_mismatch") {
				t.Fatalf("err=%v", err)
			}
		})
	}
	issue, err := PrepareIssueIdentity("https://gitlab.example/a/-/issues/7", gitlab("https://gitlab.example/a/-/issues/7"))
	if err != nil || issue.Provider != "gitlab" || issue.Issue != 7 {
		t.Fatalf("provider must be normalised: %+v %v", issue, err)
	}
}

func TestSealIntentRejectsInvalidIdentity(t *testing.T) {
	_, intent := resumeReceiptFixture(t)
	for name, tt := range map[string]struct {
		intent preparationcontract.Intent
		issue  preparationcontract.IssueIdentity
	}{
		"empty lifecycle": {func() preparationcontract.Intent { i := intent; i.LifecycleID = ""; return i }(), preparationcontract.IssueIdentity{Provider: "github", Issue: 1}},
		"bad provider":    {intent, preparationcontract.IssueIdentity{Provider: "svn", Issue: 1}},
		"negative issue":  {intent, preparationcontract.IssueIdentity{Provider: "github", Issue: -1}},
	} {
		if _, err := SealIntent(tt.intent, tt.issue); err == nil || !strings.Contains(err.Error(), "intent_identity_mismatch") {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
}

func TestPendingKindMapsEveryStage(t *testing.T) {
	for stage, want := range map[preparationcontract.IntentStage]string{
		preparationcontract.IntentStageWorktree: "worktree_create",
		preparationcontract.IntentStageTerminal: "owner_launch",
		preparationcontract.IntentStageRun:      "owner_launch",
		preparationcontract.IntentStageRunBind:  "owner_launch",
		preparationcontract.IntentStageTask:     "owner_launch",
		preparationcontract.IntentStageDispatch: "dispatch",
		"unknown":                               "",
	} {
		if got := PendingKind(stage); got != want {
			t.Fatalf("PendingKind(%q)=%q want %q", stage, got, want)
		}
	}
}

func TestValidateReconcileSnapshotRejectsMissingPendingAndDrift(t *testing.T) {
	base := preparationcontract.Record{
		ID: "io-1", IssueURL: "https://github.com/a/b/issues/1",
		BranchPrepare: []byte(`{"a":1}`),
		Execution:     &leasecontract.Execution{Mode: "orca", Pending: &leasecontract.ExternalIntent{OperationID: "op"}},
	}
	noExecution := base
	noExecution.Execution = nil
	noPending := base
	noPending.Execution = &leasecontract.Execution{Mode: "orca"}
	for name, current := range map[string]preparationcontract.Record{"no execution": noExecution, "no pending": noPending} {
		if err := ValidateReconcileSnapshot(current, base); err == nil || !strings.Contains(err.Error(), "orca_intent_invalid") {
			t.Fatalf("%s: %v", name, err)
		}
	}
	drift := func(mutate func(*preparationcontract.Record)) preparationcontract.Record {
		next := base
		mutate(&next)
		return next
	}
	for name, expected := range map[string]preparationcontract.Record{
		"id":       drift(func(r *preparationcontract.Record) { r.ID = "io-2" }),
		"issue":    drift(func(r *preparationcontract.Record) { r.IssueURL = "https://github.com/a/b/issues/2" }),
		"evidence": drift(func(r *preparationcontract.Record) { r.BranchPrepare = []byte(`{"a":2}`) }),
		"invalid":  drift(func(r *preparationcontract.Record) { r.BranchPrepare = []byte(`{`) }),
		"pending": drift(func(r *preparationcontract.Record) {
			r.Execution = &leasecontract.Execution{Mode: "orca", Pending: &leasecontract.ExternalIntent{OperationID: "other"}}
		}),
	} {
		if err := ValidateReconcileSnapshot(base, expected); err == nil || !strings.Contains(err.Error(), "orca_intent_authority_changed") {
			t.Fatalf("%s drift: %v", name, err)
		}
	}
	absent, null := base, base
	absent.BranchPrepare, null.BranchPrepare = nil, []byte("null")
	if err := ValidateReconcileSnapshot(absent, null); err != nil {
		t.Fatalf("absent and null evidence are equivalent: %v", err)
	}
}

func TestPlannerGateReportsEveryMissingKeyAndDevilsAdvocateRules(t *testing.T) {
	if got := MissingPlannerGateKeys(preparationcontract.PlannerEvidence{}); strings.Join(got, ",") != "intent_contract,design_review,devils_advocate_review" {
		t.Fatalf("empty evidence keys=%v", got)
	}
	cleared := func(review preparationcontract.PlannerDevilsAdvocateEvidence) bool {
		return len(MissingPlannerGateKeys(preparationcontract.PlannerEvidence{
			Intent:         preparationcontract.PlannerIntentEvidence{RawRequest: "r", InterpretedIntent: "i", SuccessCriteria: []string{" ", "c"}},
			DesignReview:   preparationcontract.PlannerDesignEvidence{ProblemSummary: "p", ProposedDesign: "d", Verification: []string{"v"}, Approved: true},
			DevilsAdvocate: review,
		})) == 0
	}
	for name, tt := range map[string]struct {
		review preparationcontract.PlannerDevilsAdvocateEvidence
		want   bool
	}{
		"unrecorded":                {preparationcontract.PlannerDevilsAdvocateEvidence{ReviewedPlanDigest: "d"}, false},
		"stop not waived":           {preparationcontract.PlannerDevilsAdvocateEvidence{Verdict: "stop", RecordedAt: "t", ReviewedPlanDigest: "d"}, false},
		"revise not waived":         {preparationcontract.PlannerDevilsAdvocateEvidence{Verdict: " revise ", RecordedAt: "t", ReviewedPlanDigest: "d"}, false},
		"stop waived":               {preparationcontract.PlannerDevilsAdvocateEvidence{Verdict: "stop", Waived: true, RecordedAt: "t", ReviewedPlanDigest: "d"}, true},
		"missing digest":            {preparationcontract.PlannerDevilsAdvocateEvidence{Verdict: "pass", RecordedAt: "t"}, false},
		"delegated skips digest":    {preparationcontract.PlannerDevilsAdvocateEvidence{Verdict: "pass", RecordedAt: "t", ReviewerPattern: "delegated-parent-review"}, true},
		"delegated still must stop": {preparationcontract.PlannerDevilsAdvocateEvidence{Verdict: "stop", RecordedAt: "t", ReviewerPattern: "delegated-parent-review"}, false},
	} {
		if got := cleared(tt.review); got != tt.want {
			t.Fatalf("%s: cleared=%v want %v", name, got, tt.want)
		}
	}
	onlyBlankCriteria := preparationcontract.PlannerEvidence{
		Intent: preparationcontract.PlannerIntentEvidence{RawRequest: "r", InterpretedIntent: "i", SuccessCriteria: []string{"  "}},
	}
	if got := MissingPlannerGateKeys(onlyBlankCriteria); got[0] != "intent_contract" {
		t.Fatalf("blank criteria must not count: %v", got)
	}
}

func TestOrcaStageRejectsInvalidModesAndWorktreeEvidence(t *testing.T) {
	if _, err := ValidateOrcaStagePrerequisites(OrcaStageFacts{Mode: "other", Stage: "dispatch"}); err == nil {
		t.Fatal("unknown mode accepted")
	}
	if _, err := ValidateOrcaStagePrerequisites(OrcaStageFacts{Mode: OrcaStageInvocation, Stage: "bogus"}); err == nil {
		t.Fatal("unknown stage accepted")
	}
	if err := ValidateOrcaStageReceipts(OrcaStageFacts{Stage: "bogus"}); err == nil {
		t.Fatal("unknown stage receipts accepted")
	}
	if need, err := ValidateOrcaStagePrerequisites(OrcaStageFacts{Mode: OrcaStageInvocation, Stage: "worktree_create"}); need || err != nil {
		t.Fatalf("worktree needs no sealed evidence: %v %v", need, err)
	}
	for name, facts := range map[string]OrcaStageFacts{
		"prepared": {Stage: "worktree_create", Prepared: true},
		"launch":   {Stage: "worktree_create", Launch: true},
	} {
		if err := ValidateOrcaStageReceipts(facts); err == nil || !strings.Contains(err.Error(), "later-stage receipt") {
			t.Fatalf("worktree intent with %s evidence: %v", name, err)
		}
	}
}

func TestCleanupAbandonIntentStagesTaskPrefix(t *testing.T) {
	got, err := CleanupAbandonIntentStages(preparationcontract.IntentStageTask)
	want := []preparationcontract.IntentStage{preparationcontract.IntentStageWorktree, preparationcontract.IntentStageTerminal, preparationcontract.IntentStageTask}
	if err != nil || len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestApplyResumeReceiptRejectsInvalidTransitions(t *testing.T) {
	record, intent := resumeReceiptFixture(t)
	receipt := preparationcontract.IntentReceipt{}

	prepare := intent
	prepare.Purpose = preparationcontract.PurposePrepare
	released := record
	released.Execution = &leasecontract.Execution{}
	*released.Execution = *record.Execution
	released.Execution.Lease.Status = "released"
	released.Execution.Orca = nil
	if _, err := ApplyResumeReceipt(released, prepare, receipt, nil); err == nil || !strings.Contains(err.Error(), "resume intent is required") {
		t.Fatalf("prepare intent must be rejected as non-resume: %v", err)
	}

	stale := intent
	stale.Probe.Issue++
	if _, err := ApplyResumeReceipt(record, stale, receipt, nil); err == nil {
		t.Fatal("issue identity drift accepted")
	}
	foreign := intent
	foreign.OperationID = "other"
	if _, err := ApplyResumeReceipt(record, foreign, receipt, nil); err == nil || !strings.Contains(err.Error(), "authority changed") {
		t.Fatalf("operation drift: %v", err)
	}

	worktree := intent
	worktree.Stage = preparationcontract.IntentStageWorktree
	worktreeRecord := ApplyResumeIntent(record, worktree)
	if _, err := ApplyResumeReceipt(worktreeRecord, worktree, receipt, nil); err == nil || !strings.Contains(err.Error(), "cannot create a worktree") {
		t.Fatalf("worktree stage in resume: %v", err)
	}

	unknown := intent
	unknown.Stage = "bogus"
	if _, err := ApplyResumeReceipt(ApplyResumeIntent(record, unknown), unknown, receipt, nil); err == nil {
		t.Fatal("unknown stage accepted")
	}
}

func TestApplyResumeReceiptStageCandidatesMustBeComplete(t *testing.T) {
	record, intent := resumeReceiptFixture(t)
	at := func(stage preparationcontract.IntentStage, mutate func(*preparationcontract.Intent)) (preparationcontract.Record, preparationcontract.Intent) {
		next := intent
		next.Stage = stage
		mutate(&next)
		return ApplyResumeIntent(record, next), next
	}

	r, i := at(preparationcontract.IntentStageRun, func(*preparationcontract.Intent) {})
	if _, err := ApplyResumeReceipt(r, i, preparationcontract.IntentReceipt{RunID: "  "}, nil); err == nil || !strings.Contains(err.Error(), "Run candidate is incomplete") {
		t.Fatalf("blank run id: %v", err)
	}
	next, err := ApplyResumeReceipt(r, i, preparationcontract.IntentReceipt{RunID: " run-1 "}, nil)
	if err != nil || next.Intent.RunID != "run-1" || next.Intent.Stage != preparationcontract.IntentStageRunBind || next.Record.Execution.Pending.Kind != "owner_launch" {
		t.Fatalf("run receipt: %+v %v", next, err)
	}

	r, i = at(preparationcontract.IntentStageRunBind, func(in *preparationcontract.Intent) { in.RunID = "run-1" })
	for name, receipt := range map[string]preparationcontract.IntentReceipt{
		"unbound":   {RunID: "run-1"},
		"other run": {RunID: "run-2", RunBound: true},
	} {
		if _, err := ApplyResumeReceipt(r, i, receipt, nil); err == nil || !strings.Contains(err.Error(), "Run binding candidate is incomplete") {
			t.Fatalf("%s: %v", name, err)
		}
	}
	next, err = ApplyResumeReceipt(r, i, preparationcontract.IntentReceipt{RunID: " run-1 ", RunBound: true}, nil)
	if err != nil || !next.Intent.RunBound || next.Intent.Stage != preparationcontract.IntentStageTask {
		t.Fatalf("bind receipt: %+v %v", next, err)
	}

	r, i = at(preparationcontract.IntentStageTask, func(*preparationcontract.Intent) {})
	if _, err := ApplyResumeReceipt(r, i, preparationcontract.IntentReceipt{}, nil); err == nil || !strings.Contains(err.Error(), "task candidate is incomplete") {
		t.Fatalf("blank task id: %v", err)
	}
	next, err = ApplyResumeReceipt(r, i, preparationcontract.IntentReceipt{TaskID: " task-1 "}, nil)
	if err != nil || next.Intent.TaskID != "task-1" || next.Intent.Stage != preparationcontract.IntentStageDispatch {
		t.Fatalf("task receipt: %+v %v", next, err)
	}
}

func TestApplyResumeDispatchReceiptRequiresDeliveryAndSealedArtifacts(t *testing.T) {
	record, intent := resumeReceiptFixture(t)
	intent.Stage = preparationcontract.IntentStageDispatch
	intent.TerminalPTYID, intent.RunID, intent.RunBound, intent.TaskID = "pty", "run", true, "task"
	record = ApplyResumeIntent(record, intent)

	deliveryErr := errors.New("delivery refused")
	if _, err := ApplyResumeReceipt(record, intent, preparationcontract.IntentReceipt{DispatchID: "d"}, deliveryErr); !errors.Is(err, deliveryErr) {
		t.Fatalf("delivery failure must be wrapped: %v", err)
	}

	noLaunch := intent
	noLaunch.Launch = nil
	if _, err := ApplyResumeReceipt(record, noLaunch, preparationcontract.IntentReceipt{DispatchID: "d"}, nil); err == nil {
		t.Fatal("dispatch without launch identity accepted")
	}
	noPrepared := intent
	noPrepared.Prepared = nil
	if _, err := ApplyResumeReceipt(record, noPrepared, preparationcontract.IntentReceipt{DispatchID: "d"}, nil); err == nil {
		t.Fatal("dispatch without prepared receipt accepted")
	}

	next, err := ApplyResumeReceipt(record, intent, preparationcontract.IntentReceipt{
		DispatchID: "d", RequestID: " req ", PromptReceipt: &preparationcontract.PromptReceipt{RequestID: " prompt-req "},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if next.Intent.OrcaRequestID != "req" || next.Intent.OrcaPromptRequestID != "prompt-req" ||
		next.Record.Execution.Orca.LeaseGeneration != intent.Generation || next.Record.Execution.Orca.OwnerPromptSHA256 != intent.Launch.PromptSHA256 {
		t.Fatalf("dispatch receipt not bound: %+v", next)
	}
}

func TestBuildResumeIntentValidationAndReusedTerminal(t *testing.T) {
	record, _ := resumeReceiptFixture(t)
	input := ResumeIntentInput{
		Record: record, Workspace: preparationcontract.WorkspaceRequest{LifecycleID: record.ID, SourceRoot: "/repo", Root: "/repo.worktrees/193-fix", Branch: "193-fix", BaseHead: "base", Confirm: true},
		OperationID: strings.Repeat("e", 32), StartedAt: "2026-07-31T03:15:00Z",
		Artifacts: preparationcontract.ResumeArtifacts{IssueBodySHA256: strings.Repeat("a", 64), OwnerPromptPath: "/p", OwnerPromptSHA256: strings.Repeat("c", 64), ContextPacketPath: "/c", ContextPacketSHA256: strings.Repeat("d", 64)},
		RuntimeID: "rt",
	}
	input.Record.Execution = nil
	if _, err := BuildResumeIntent(input); err == nil || !strings.Contains(err.Error(), "existing Orca binding") {
		t.Fatalf("no execution: %v", err)
	}
	input.Record = record
	unbound := record
	unbound.Execution = &leasecontract.Execution{}
	*unbound.Execution = *record.Execution
	unbound.Execution.Orca = nil
	input.Record = unbound
	if _, err := BuildResumeIntent(input); err == nil || !strings.Contains(err.Error(), "existing Orca binding") {
		t.Fatalf("no binding: %v", err)
	}
	input.Record = record
	input.Record.BranchPrepare = nil
	if _, err := BuildResumeIntent(input); err == nil || !strings.Contains(err.Error(), "intent_identity_mismatch") {
		t.Fatalf("no branch evidence: %v", err)
	}
	input.Record = record
	input.ReusedTerminalPTYID = " pty-kept "
	intent, err := BuildResumeIntent(input)
	if err != nil || intent.Stage != preparationcontract.IntentStageRun || intent.TerminalPTYID != "pty-kept" {
		t.Fatalf("reused terminal must start at the Run stage: %+v %v", intent, err)
	}
}

func TestResumeWorkspaceRejectsInvalidInputs(t *testing.T) {
	valid := func() preparationcontract.Record {
		return preparationcontract.Record{ID: "io-1", Repo: "/repo", Branch: "193/fix", BranchPrepare: []byte(`{"base_branch":"main","base_sha":"head"}`)}
	}
	for name, mutate := range map[string]func(*preparationcontract.Record){
		"invalid json": func(r *preparationcontract.Record) { r.BranchPrepare = []byte(`{`) },
		"blank branch": func(r *preparationcontract.Record) { r.Branch = "  " },
		"dot branch":   func(r *preparationcontract.Record) { r.Branch = ".." },
		"delegated no base": func(r *preparationcontract.Record) {
			r.Delegation = []byte(`{"parent_cycle_id":"io-0"}`)
			r.BranchPrepare = []byte(`{"base_sha":"head"}`)
		},
		"parent dot base": func(r *preparationcontract.Record) {
			r.BranchPrepare = []byte(`{"base_branch":".","base_sha":"head","parent_worktree":"/p"}`)
		},
	} {
		record := valid()
		mutate(&record)
		if _, err := ResumeWorkspace(record); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	record := valid()
	record.Delegation = []byte(`{"parent_cycle_id":"io-0"}`)
	plan, err := ResumeWorkspace(record)
	if err != nil || plan.ExpectedParent != "/repo.worktrees/main" || plan.Request.ParentWorktree != plan.ExpectedParent {
		t.Fatalf("delegated parent without explicit worktree: %+v %v", plan, err)
	}
}

func TestCanonicalizeIntentRejectsUndecodablePayload(t *testing.T) {
	record, _ := resumeReceiptFixture(t)
	intent, raw, err := CanonicalizeIntent(record, []byte(`{not json`))
	if err == nil || raw != nil || intent.LifecycleID != "" {
		t.Fatalf("undecodable payload accepted: %+v %v", intent, err)
	}
}

func TestSealIntentRejectsIntentThatFailsCodecValidation(t *testing.T) {
	_, intent := resumeReceiptFixture(t)
	intent.StartedAt = ""
	if _, err := SealIntent(intent, preparationcontract.IssueIdentity{Provider: "github", Issue: 193}); err == nil {
		t.Fatal("intent without start time was sealed")
	}
}

func TestValidateReconcileIntentIssueIdentityRejectsMissingEvidenceAndLifecycleDrift(t *testing.T) {
	record := preparationcontract.Record{
		ID: "io-1", IssueURL: "https://github.com/a/b/issues/1",
		BranchPrepare: []byte(`{"provider":"github","issue_url":"https://github.com/a/b/issues/1","link_verified":true}`),
	}
	intent := preparationcontract.Intent{LifecycleID: "io-1", Probe: preparationcontract.ProbeRequest{Provider: "github", Issue: 1}}
	if err := ValidateReconcileIntentIssueIdentity(record, intent); err != nil {
		t.Fatal(err)
	}
	other := intent
	other.LifecycleID = "io-2"
	if err := ValidateReconcileIntentIssueIdentity(record, other); err == nil || !strings.Contains(err.Error(), "intent_identity_mismatch") {
		t.Fatalf("lifecycle drift: %v", err)
	}
	record.BranchPrepare = nil
	if err := ValidateReconcileIntentIssueIdentity(record, intent); err == nil {
		t.Fatal("missing branch evidence accepted")
	}
}
