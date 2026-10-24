package issueops

import (
	"errors"
	"slices"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func covCFAttempt(op model.CleanupOperation, tokenChar string) model.IssueOpsCleanupAttempt {
	return model.IssueOpsCleanupAttempt{Operation: op, Token: strings.Repeat(tokenChar, 64), StartedAt: "2026-01-02T03:04:05Z"}
}

func TestCovCFCleanupOperationAccessRejectsForeignOperation(t *testing.T) {
	attempt := covCFAttempt(model.CleanupOperationFinish, "a")
	record := model.IssueOpsRecord{CleanupAttempt: &attempt}
	err := ValidateCleanupOperationAccess(record, model.CleanupOperationAbandon)
	if err == nil || !strings.Contains(err.Error(), "resume cleanup finish before cleanup abandon") {
		t.Fatalf("err = %v", err)
	}
	if err := ValidateCleanupOperationAccess(record, model.CleanupOperationFinish); err != nil {
		t.Fatalf("same operation must pass: %v", err)
	}
	if err := ValidateCleanupOperationAccess(model.IssueOpsRecord{}, model.CleanupOperationFinish); err != nil {
		t.Fatalf("no attempt must pass: %v", err)
	}
}

func TestCovCFArmCleanupRejections(t *testing.T) {
	base := covCFAttempt(model.CleanupOperationFinish, "a")

	if _, err := ArmCleanup(model.IssueOpsRecord{}, model.IssueOpsCleanupAttempt{Operation: "bogus", Token: base.Token, StartedAt: base.StartedAt}); err == nil {
		t.Fatal("invalid attempt must be rejected")
	}

	foreign := covCFAttempt(model.CleanupOperationAbandon, "b")
	if _, err := ArmCleanup(model.IssueOpsRecord{CleanupAttempt: &foreign}, base); err == nil || !strings.Contains(err.Error(), "owns this cycle") {
		t.Fatalf("foreign owner err = %v", err)
	}

	applying := model.IssueOpsRecord{CleanupAbandonFailure: &model.IssueOpsCleanupAbandonFailure{Step: model.CleanupFailureStepApplying}}
	if _, err := ArmCleanup(applying, base); err == nil || !strings.Contains(err.Error(), "abandon apply is in progress") {
		t.Fatalf("abandon-applying err = %v", err)
	}
	// The abandon operation itself may proceed through its own applying failure.
	abandon := covCFAttempt(model.CleanupOperationAbandon, "c")
	if got, err := ArmCleanup(applying, abandon); err != nil || got.CleanupAttempt == nil || *got.CleanupAttempt != abandon {
		t.Fatalf("abandon resume = %+v, %v", got.CleanupAttempt, err)
	}

	same := model.IssueOpsRecord{CleanupAttempt: &base}
	if _, err := ArmCleanup(same, base); err == nil || !strings.Contains(err.Error(), "requires a new attempt token") {
		t.Fatalf("reused token err = %v", err)
	}
}

func TestCovCFArmCleanupSetsAttemptAndTimestampThenRelease(t *testing.T) {
	attempt := covCFAttempt(model.CleanupOperationRemoteBranch, "d")
	armed, err := ArmCleanup(model.IssueOpsRecord{UpdatedAt: "old"}, attempt)
	if err != nil {
		t.Fatal(err)
	}
	if armed.CleanupAttempt == nil || *armed.CleanupAttempt != attempt || armed.UpdatedAt != attempt.StartedAt {
		t.Fatalf("armed = %+v", armed)
	}
	if err := ValidateCleanupOwner(armed, attempt); err != nil {
		t.Fatalf("owner must match: %v", err)
	}
	released := ReleaseCleanupAttempt(armed, "2027-01-01T00:00:00Z")
	if released.CleanupAttempt != nil || released.UpdatedAt != "2027-01-01T00:00:00Z" {
		t.Fatalf("released = %+v", released)
	}
	if armed.CleanupAttempt == nil {
		t.Fatal("release must not mutate the caller's copy")
	}
}

func TestCovCFValidateCleanupOwnerRejections(t *testing.T) {
	attempt := covCFAttempt(model.CleanupOperationFinish, "a")
	other := covCFAttempt(model.CleanupOperationFinish, "b")
	cases := map[string]struct {
		record   model.IssueOpsRecord
		expected model.IssueOpsCleanupAttempt
	}{
		"empty token":     {model.IssueOpsRecord{CleanupAttempt: &attempt}, model.IssueOpsCleanupAttempt{}},
		"no attempt":      {model.IssueOpsRecord{}, attempt},
		"different token": {model.IssueOpsRecord{CleanupAttempt: &other}, attempt},
	}
	for name, tc := range cases {
		if err := ValidateCleanupOwner(tc.record, tc.expected); err == nil || !strings.Contains(err.Error(), "ownership changed") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestCovCFCleanupCommandGenerationAndProvenance(t *testing.T) {
	if got := CleanupCommandGeneration(model.IssueOpsRecord{}); got != 0 {
		t.Fatalf("nil execution = %d", got)
	}
	record := model.IssueOpsRecord{Execution: &model.Execution{Lease: model.WriteLease{Generation: 7}}}
	if got := CleanupCommandGeneration(record); got != 7 {
		t.Fatalf("generation = %d", got)
	}
	cases := []struct {
		command string
		gen     uint64
		want    bool
	}{{"git status", 1, true}, {"  \t", 1, false}, {"", 3, false}, {"git status", 0, false}}
	for _, tc := range cases {
		if got := CleanupCommandNeedsProvenance(tc.command, tc.gen); got != tc.want {
			t.Errorf("(%q,%d) = %v", tc.command, tc.gen, got)
		}
	}
}

func TestCovCFValidateChildCleanupMergeObservation(t *testing.T) {
	artifact := func() *model.IssueOpsRemoteArtifactVerification {
		return &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://x/1", TargetBranch: "main"}
	}
	base := func() model.IssueOpsRecord {
		return model.IssueOpsRecord{ID: "c", Repo: "r", Branch: "b", IssueURL: "u", RemoteArtifact: artifact(),
			IssueLinks: []model.IssueOpsIssueLink{{Type: "child", URL: "c1"}}}
	}
	if err := ValidateChildCleanupMergeObservation(base(), base()); err != nil {
		t.Fatalf("identical must pass: %v", err)
	}
	unrelated := base()
	unrelated.UpdatedAt = "later"
	unrelated.RemoteArtifact.VerifiedAt = "later"
	unrelated.RemoteArtifact.Labels = []string{"x"}
	if err := ValidateChildCleanupMergeObservation(base(), unrelated); err != nil {
		t.Fatalf("unrelated metadata must pass: %v", err)
	}
	mutations := map[string]func(*model.IssueOpsRecord){
		"id":            func(r *model.IssueOpsRecord) { r.ID = "d" },
		"repo":          func(r *model.IssueOpsRecord) { r.Repo = "r2" },
		"branch":        func(r *model.IssueOpsRecord) { r.Branch = "b2" },
		"issue url":     func(r *model.IssueOpsRecord) { r.IssueURL = "u2" },
		"links":         func(r *model.IssueOpsRecord) { r.IssueLinks[0].CloseVerifiedAt = "t" },
		"artifact gone": func(r *model.IssueOpsRecord) { r.RemoteArtifact = nil },
		"provider":      func(r *model.IssueOpsRecord) { r.RemoteArtifact.Provider = "gitlab" },
		"kind":          func(r *model.IssueOpsRecord) { r.RemoteArtifact.Kind = "mr" },
		"url":           func(r *model.IssueOpsRecord) { r.RemoteArtifact.URL = "https://x/2" },
		"target branch": func(r *model.IssueOpsRecord) { r.RemoteArtifact.TargetBranch = "dev" },
	}
	for name, mutate := range mutations {
		current := base()
		mutate(&current)
		if err := ValidateChildCleanupMergeObservation(base(), current); err == nil || !strings.Contains(err.Error(), "changed during merge observation") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	noArtifact := base()
	noArtifact.RemoteArtifact = nil
	if err := ValidateChildCleanupMergeObservation(noArtifact, base()); err == nil {
		t.Error("artifact appearing must be detected")
	}
	if err := ValidateChildCleanupMergeObservation(noArtifact, noArtifact); err != nil {
		t.Errorf("both nil artifacts must pass: %v", err)
	}
}

func TestCovCFPreparedBaseNormalization(t *testing.T) {
	if got := PreparedBaseBranch(model.IssueOpsRecord{}); got != "" {
		t.Fatalf("no prepare = %q", got)
	}
	if got := PreparedBaseRef(model.IssueOpsRecord{}); got != "" {
		t.Fatalf("no prepare ref = %q", got)
	}
	cases := map[string]string{"main": "main", " main ": "main", "refs/heads/main": "main", "origin/main": "main", "refs/heads/origin/dev": "dev", "feature/x": "feature/x"}
	for in, want := range cases {
		record := model.IssueOpsRecord{BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: in}}
		if got := PreparedBaseRef(record); got != want {
			t.Errorf("PreparedBaseRef(%q) = %q want %q", in, got, want)
		}
	}
	record := model.IssueOpsRecord{BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: "  origin/main "}}
	if got := PreparedBaseBranch(record); got != "origin/main" {
		t.Fatalf("PreparedBaseBranch = %q", got)
	}
}

func TestCovCFValidateCleanupSupersedeInput(t *testing.T) {
	withURL := model.IssueOpsRecord{RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "https://x/1"}}
	blankURL := model.IssueOpsRecord{RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "  "}}
	cases := []struct {
		name       string
		record     model.IssueOpsRecord
		candidate  string
		configured bool
		want       string
	}{
		{"blank candidate", withURL, "  ", true, "no superseding artifact"},
		{"not configured", withURL, "https://x/2", false, "provider observation is not configured"},
		{"no artifact", model.IssueOpsRecord{}, "https://x/2", true, "original artifact URL is unknown"},
		{"blank artifact url", blankURL, "https://x/2", true, "original artifact URL is unknown"},
		{"ok", withURL, "https://x/2", true, ""},
	}
	for _, tc := range cases {
		err := ValidateCleanupSupersedeInput(tc.record, tc.candidate, tc.configured)
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v want %q", tc.name, err, tc.want)
		}
	}
}

func TestCovCFFinishCommands(t *testing.T) {
	if got := CleanupFinishRemedyCommand("c1", []string{"phase_done", "completion_reflected"}); got != "issueops remote reflect-completion --id c1 --body-file <PROGRESS_REPORT_FILE> --confirm --json" {
		t.Fatalf("remedy = %q", got)
	}
	if got := CleanupFinishRemedyCommand("c1", []string{"phase_done"}); got != "" {
		t.Fatalf("unrelated remedy = %q", got)
	}
	// The remedy follows the cleanup order an agent walks: child tasks,
	// remote branch, completion reflection, issue close.
	all := []string{"issue_closed", "completion_reflected", "remote_branch_absent", "child_tasks_closed"}
	for _, want := range []string{"cleanup close-children --id c1 --merged", "cleanup remote-branch --id c1 --preview", "remote reflect-completion --id c1", "remote close-issue --id c1 --json"} {
		if got := CleanupFinishRemedyCommand("c1", all); !strings.Contains(got, want) {
			t.Fatalf("remedy for %q = %q, want %q", all, got, want)
		}
		all = all[:len(all)-1]
	}
	if got := CleanupFinishApplyCommand("c1", "fp", "", false); got != "issueops cleanup finish --id c1 --apply --confirm --fingerprint fp --json" {
		t.Fatalf("plain = %q", got)
	}
	got := CleanupFinishApplyCommand("c1", "fp", " it's ", true)
	want := `issueops cleanup finish --id c1 --apply --confirm --fingerprint fp --superseded-by 'it'\''s' --keep-remote-branch --json`
	if got != want {
		t.Fatalf("quoted = %q want %q", got, want)
	}
	if got := CleanupFinishApplyCommand("c1", "fp", "   ", false); strings.Contains(got, "--superseded-by") {
		t.Fatalf("blank superseded-by must be omitted: %q", got)
	}
}

func TestCovCFValidateApplyRequiresConfirmAndFingerprint(t *testing.T) {
	if err := ValidateCleanupFinishApply(model.CleanupFinishRequest{Fingerprint: "fp"}, "fp"); err == nil || !strings.Contains(err.Error(), "requires --confirm") {
		t.Fatalf("finish unconfirmed = %v", err)
	}
	if err := ValidateCleanupFinishApply(model.CleanupFinishRequest{Confirm: true, Fingerprint: "old"}, "fp"); err == nil || !strings.Contains(err.Error(), "stale cleanup fingerprint") {
		t.Fatalf("finish stale = %v", err)
	}
	if err := ValidateCleanupFinishApply(model.CleanupFinishRequest{Confirm: true, Fingerprint: "fp"}, "fp"); err != nil {
		t.Fatalf("finish ok = %v", err)
	}
	if err := ValidateCleanupRemoteBranchApply(model.CleanupRemoteBranchRequest{Fingerprint: "fp"}, "fp"); err == nil || !strings.Contains(err.Error(), "requires --confirm") {
		t.Fatalf("remote unconfirmed = %v", err)
	}
	if err := ValidateCleanupRemoteBranchApply(model.CleanupRemoteBranchRequest{Confirm: true, Fingerprint: "old"}, "fp"); err == nil || !strings.Contains(err.Error(), "stale cleanup fingerprint") {
		t.Fatalf("remote stale = %v", err)
	}
	if err := ValidateCleanupRemoteBranchApply(model.CleanupRemoteBranchRequest{Confirm: true, Fingerprint: "fp"}, "fp"); err != nil {
		t.Fatalf("remote ok = %v", err)
	}
}

func TestCovCFFinishAudit(t *testing.T) {
	inv := model.CleanupFinishInventory{WorktreeRoot: "/wt", Branch: "feat", BranchOID: "abc"}
	res := model.CleanupFinishResult{
		WorkspaceProcessesStopped: []model.CleanupWorkspaceProcess{{PID: 1}, {PID: 2}},
	}
	if got, want := CleanupFinishAudit(inv, res, "T"), "cleanup 완료: worktree=/wt branch=feat oid=abc stopped=2 at=T"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	res.KeptRemoteBranch = &model.CleanupKeptRemoteBranch{Branch: "feat", RemoteOID: "deadbeef", State: model.CleanupKeptRemoteBranchPresent}
	if got := CleanupFinishAudit(inv, res, "T"); !strings.Contains(got, " remote_branch_kept=feat@deadbeef at=T") {
		t.Fatalf("kept present = %q", got)
	}
	res.KeptRemoteBranch = &model.CleanupKeptRemoteBranch{Branch: "feat", State: model.CleanupKeptRemoteBranchUnreadable}
	if got := CleanupFinishAudit(inv, res, "T"); !strings.Contains(got, "remote_branch_kept=feat@unreadable") {
		t.Fatalf("kept unreadable = %q", got)
	}
	empty := CleanupFinishAudit(model.CleanupFinishInventory{WorktreeRoot: " ", Branch: ""}, model.CleanupFinishResult{}, "T")
	if empty != "cleanup 완료: worktree=(없음) branch=(없음) oid=(없음) stopped=0 at=T" {
		t.Fatalf("empty = %q", empty)
	}
}

func TestCovCFFinishPreviewBaseMissingBranches(t *testing.T) {
	record := model.IssueOpsRecord{ID: "c", Phase: model.IssueOpsPhaseDone, BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: "main"}}
	req := model.CleanupFinishRequest{Merged: true, CompletionReflected: true, IssueClosed: true}

	unobserved := BuildCleanupFinishPreview(record, req, model.CleanupFinishInventory{}, CleanupFinishObservation{})
	if !slices.Equal(unobserved.Missing, []string{"merged_base_branch_unobserved"}) || unobserved.OK {
		t.Fatalf("unobserved = %+v", unobserved)
	}

	// A superseding artifact bypasses base classification entirely.
	req.MergedBaseBranch = "other"
	superseded := BuildCleanupFinishPreview(record, req, model.CleanupFinishInventory{SupersededBy: "https://x/2"}, CleanupFinishObservation{BaseObserved: true, DefaultBranch: "main", PreparedBasePresent: true})
	if !superseded.OK || superseded.RetargetedBase != nil {
		t.Fatalf("superseded = %+v", superseded)
	}

	// Drift with a present prepared base is blocking.
	drift := BuildCleanupFinishPreview(record, req, model.CleanupFinishInventory{}, CleanupFinishObservation{BaseObserved: true, DefaultBranch: "main", PreparedBasePresent: true})
	if !slices.Equal(drift.Missing, []string{"base_branch_drifted"}) || drift.RetargetedBase != nil {
		t.Fatalf("drift = %+v", drift)
	}

	// Retarget to the default branch after the prepared base disappeared is recorded.
	req.MergedBaseBranch = "develop"
	retarget := BuildCleanupFinishPreview(record, req, model.CleanupFinishInventory{}, CleanupFinishObservation{BaseObserved: true, DefaultBranch: "develop"})
	want := &model.CleanupRetargetedBase{PreparedBase: "main", ObservedBase: "develop", DefaultBranch: "develop", PreparedBaseRemoteAbsent: true}
	if !retarget.OK || retarget.RetargetedBase == nil || *retarget.RetargetedBase != *want {
		t.Fatalf("retarget = %+v", retarget)
	}

	// Observed == prepared needs no retarget record.
	req.MergedBaseBranch = "main"
	same := BuildCleanupFinishPreview(record, req, model.CleanupFinishInventory{}, CleanupFinishObservation{BaseObserved: true})
	if !same.OK || same.RetargetedBase != nil {
		t.Fatalf("same = %+v", same)
	}
}

func TestCovCFFinishPreviewWorktreeGates(t *testing.T) {
	record := model.IssueOpsRecord{ID: "c", Phase: model.IssueOpsPhaseDone}
	req := model.CleanupFinishRequest{Merged: true, CompletionReflected: true, IssueClosed: true}
	inv := model.CleanupFinishInventory{WorktreePresent: true, WorktreeRoot: "/wt"}

	got := BuildCleanupFinishPreview(record, req, inv, CleanupFinishObservation{})
	if !slices.Equal(got.Missing, []string{"cwd_unresolved", "worktree_clean"}) {
		t.Fatalf("no cwd, dirty = %v", got.Missing)
	}
	req.CWD = "/elsewhere"
	got = BuildCleanupFinishPreview(record, req, inv, CleanupFinishObservation{CWDWithin: true, WorktreeClean: true, WorkspaceMissing: []string{"workspace_x"}})
	if !slices.Equal(got.Missing, []string{"cwd_outside_worktree", "workspace_x"}) {
		t.Fatalf("cwd within = %v", got.Missing)
	}
	got = BuildCleanupFinishPreview(record, req, inv, CleanupFinishObservation{WorktreeClean: true, WorktreeIdentityConflict: true, WorktreeUnobservable: true})
	if !slices.Equal(got.Missing, []string{"worktree_identity_conflict", "worktree_observable"}) {
		t.Fatalf("identity = %v", got.Missing)
	}
	// Worktree gates are skipped when the worktree is absent.
	got = BuildCleanupFinishPreview(record, req, model.CleanupFinishInventory{}, CleanupFinishObservation{})
	if !got.OK {
		t.Fatalf("absent worktree = %+v", got)
	}
}

func TestCovCFFinishPreviewBranchGates(t *testing.T) {
	record := model.IssueOpsRecord{ID: "c", Phase: model.IssueOpsPhaseDone}
	req := model.CleanupFinishRequest{Merged: true, CompletionReflected: true, IssueClosed: true}
	inv := model.CleanupFinishInventory{Branch: "feat", BranchOID: "abc"}

	got := BuildCleanupFinishPreview(record, req, inv, CleanupFinishObservation{})
	if !slices.Equal(got.Missing, []string{"local_branch_observable", "remote_branch_absent"}) || got.KeptRemoteBranch != nil || !got.BranchPresent {
		t.Fatalf("unobservable = %+v", got)
	}
	got = BuildCleanupFinishPreview(record, req, inv, CleanupFinishObservation{BranchObservable: true, RemoteReadable: true})
	if !got.OK || got.KeptRemoteBranch != nil {
		t.Fatalf("remote absent = %+v", got)
	}
	got = BuildCleanupFinishPreview(record, req, inv, CleanupFinishObservation{BranchObservable: true, RemoteReadable: true, RemoteOID: "tip"})
	if !slices.Equal(got.Missing, []string{"remote_branch_absent"}) {
		t.Fatalf("remote present = %v", got.Missing)
	}
	req.KeepRemoteBranch = true
	got = BuildCleanupFinishPreview(record, req, inv, CleanupFinishObservation{BranchObservable: true, RemoteReadable: true, RemoteOID: "tip"})
	if !got.OK || got.KeptRemoteBranch == nil || *got.KeptRemoteBranch != (model.CleanupKeptRemoteBranch{Branch: "feat", RemoteOID: "tip", State: model.CleanupKeptRemoteBranchPresent}) {
		t.Fatalf("keep present = %+v", got)
	}
	got = BuildCleanupFinishPreview(record, req, inv, CleanupFinishObservation{BranchObservable: true})
	if !got.OK || got.KeptRemoteBranch == nil || *got.KeptRemoteBranch != (model.CleanupKeptRemoteBranch{Branch: "feat", State: model.CleanupKeptRemoteBranchUnreadable}) {
		t.Fatalf("keep unreadable = %+v", got)
	}
}

func TestCovCFFinishTargetsPrefersExecutionWorkspace(t *testing.T) {
	record := model.IssueOpsRecord{ID: "c", Repo: "r", Branch: " rec ", WorktreePath: " /legacy ",
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "https://x/1"},
		Execution: &model.Execution{
			Workspace: model.Workspace{Root: " /exec ", Branch: " exec-branch "},
		}}
	got := CleanupFinishTargets(record)
	if got.WorktreeRoot != "/exec" || got.Branch != "exec-branch" || got.RemoteURL != "https://x/1" {
		t.Fatalf("targets = %+v", got)
	}
	record.Execution.Workspace = model.Workspace{}
	got = CleanupFinishTargets(record)
	if got.WorktreeRoot != "/legacy" || got.Branch != "rec" {
		t.Fatalf("fallback = %+v", got)
	}
}

func TestCovCFRemoteBranchPreviewRecordGates(t *testing.T) {
	merged := CleanupRemoteBranchObservation{MergeConfigured: true, Head: model.CleanupRemoteBranchArtifactHead{HeadRefName: "12-feat"}}
	record := model.IssueOpsRecord{ID: "c", Phase: model.IssueOpsPhaseDone, RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "u"}}

	empty := model.CleanupRemoteBranchInventory{}
	_, got := BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, empty, merged)
	if !slices.Contains(got.Missing, "branch_recorded") || !slices.Contains(got.Missing, "artifact_head_branch_match") {
		t.Fatalf("empty branch = %v", got.Missing)
	}

	_, got = BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, model.CleanupRemoteBranchInventory{Branch: "bad branch"}, merged)
	if !slices.Contains(got.Missing, "branch_name_revalidated") {
		t.Fatalf("invalid name = %v", got.Missing)
	}

	record.BranchPrepare = &model.IssueOpsBranchPrepare{BaseBranch: " 12-feat "}
	_, got = BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, model.CleanupRemoteBranchInventory{Branch: "12-feat"}, merged)
	if !slices.Contains(got.Missing, "branch_not_base") {
		t.Fatalf("base collision = %v", got.Missing)
	}

	record.BranchPrepare = nil
	record.Phase = model.IssueOpsPhasePR
	record.Execution = &model.Execution{Lease: model.WriteLease{Status: model.LeaseStatusActive}}
	record.IssueLinks = []model.IssueOpsIssueLink{{Type: "child"}}
	_, got = BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{Apply: true}, model.CleanupRemoteBranchInventory{Branch: "12-feat"}, merged)
	if got.Preview || !slices.Equal(got.Missing, []string{"phase_done", "lease_released", "child_tasks_closed"}) {
		t.Fatalf("lifecycle = %+v", got)
	}
}

func TestCovCFRemoteBranchPreviewArtifactStates(t *testing.T) {
	record := model.IssueOpsRecord{ID: "c", Phase: model.IssueOpsPhaseDone, RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "u"}}
	inv := model.CleanupRemoteBranchInventory{Branch: "12-feat"}

	noArtifact := record
	noArtifact.RemoteArtifact = nil
	_, got := BuildCleanupRemoteBranchPreview(noArtifact, model.CleanupRemoteBranchRequest{}, inv, CleanupRemoteBranchObservation{})
	if !slices.Contains(got.Missing, "remote_artifact_present") {
		t.Fatalf("no artifact = %v", got.Missing)
	}

	_, got = BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, inv, CleanupRemoteBranchObservation{})
	if !slices.Equal(got.Missing, []string{"remote_artifact_merged"}) || got.ArtifactError != "merge verification is not configured" {
		t.Fatalf("unconfigured = %+v", got)
	}

	boom := errors.New("provider down")
	facts := CleanupRemoteBranchObservation{MergeConfigured: true, MergeError: boom, ArtifactSupersedeError: errors.New("no relation")}
	_, got = BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, inv, facts)
	if !slices.Equal(got.Missing, []string{"remote_artifact_merged"}) || got.ArtifactError != "provider down" || got.SupersedeError != "no relation" {
		t.Fatalf("merge error = %+v", got)
	}

	facts.ArtifactSupersededBy = "https://x/2"
	outInv, got := BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, inv, facts)
	if !got.OK || outInv.SupersededBy != "https://x/2" || got.SupersededBy != "https://x/2" {
		t.Fatalf("superseded artifact = %+v / %+v", outInv, got)
	}

	facts = CleanupRemoteBranchObservation{MergeConfigured: true, Head: model.CleanupRemoteBranchArtifactHead{HeadRefName: "other", HeadRefOID: "o1"}, IdentityError: errors.New("wrong remote")}
	_, got = BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, inv, facts)
	if !slices.Equal(got.Missing, []string{"artifact_head_branch_match", "remote_identity_match"}) || got.RemoteIdentityError != "wrong remote" || got.ArtifactHeadBranch != "other" || got.ArtifactHeadOID != "o1" {
		t.Fatalf("head mismatch = %+v", got)
	}
}

func TestCovCFRemoteBranchPreviewTipEvidence(t *testing.T) {
	record := model.IssueOpsRecord{ID: "c", Phase: model.IssueOpsPhaseDone, RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "u"}}
	inv := model.CleanupRemoteBranchInventory{Branch: "12-feat"}
	head := model.CleanupRemoteBranchArtifactHead{HeadRefName: "12-feat", HeadRefOID: "AbC"}
	base := CleanupRemoteBranchObservation{MergeConfigured: true, Head: head, RemoteOID: "abc"}

	// Tip equal (case-insensitively) to merged head needs no extra evidence.
	outInv, got := BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, inv, base)
	if !got.OK || !got.RemoteBranchPresent || got.RemoteOID != "abc" || outInv.RemoteOID != "abc" || got.RemoteTipReachedBase {
		t.Fatalf("equal tip = %+v", got)
	}

	advanced := base
	advanced.RemoteOID = "def"
	_, got = BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, inv, advanced)
	if !slices.Equal(got.Missing, []string{"remote_tip_equals_merged_head"}) {
		t.Fatalf("advanced = %v", got.Missing)
	}

	advanced.TipSupersedeError = errors.New("no relation")
	_, got = BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, inv, advanced)
	if got.SupersedeError != "no relation" || got.OK {
		t.Fatalf("tip supersede error = %+v", got)
	}

	advanced.TipReachedBase = true
	_, got = BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, inv, advanced)
	if !got.OK || !got.RemoteTipReachedBase {
		t.Fatalf("reached base = %+v", got)
	}

	advanced.TipReachedBase = false
	advanced.TipSupersededBy = "https://x/3"
	outInv, got = BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, inv, advanced)
	if !got.OK || outInv.SupersededBy != "https://x/3" {
		t.Fatalf("tip superseded = %+v / %+v", outInv, got)
	}

	unreadable := base
	unreadable.RemoteError = errors.New("ls-remote failed")
	_, got = BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, inv, unreadable)
	if !slices.Equal(got.Missing, []string{"remote_branch_readable"}) || got.RemoteBranchPresent {
		t.Fatalf("unreadable = %+v", got)
	}

	// An unconfigured merge check means a matching tip cannot be trusted without evidence.
	if !CleanupRemoteBranchNeedsTipEvidence(CleanupRemoteBranchObservation{RemoteOID: "abc", Head: head}) {
		t.Fatal("unconfigured merge must require tip evidence")
	}
	if CleanupRemoteBranchNeedsTipEvidence(CleanupRemoteBranchObservation{}) {
		t.Fatal("absent remote branch needs no tip evidence")
	}
}

func TestCovCFRemoteBranchTargetsPreferExecutionBranch(t *testing.T) {
	record := model.IssueOpsRecord{ID: "c", Repo: "r", Branch: " rec ", RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: " https://x/1 "},
		Execution: &model.Execution{Workspace: model.Workspace{Branch: " exec "}}}
	got := CleanupRemoteBranchTargets(record)
	if got.Branch != "exec" || got.ArtifactURL != "https://x/1" {
		t.Fatalf("targets = %+v", got)
	}
	record.Execution.Workspace.Branch = ""
	if got := CleanupRemoteBranchTargets(record); got.Branch != "rec" {
		t.Fatalf("fallback = %+v", got)
	}
}

func TestCovCFStatusFinishEvidenceAndConversion(t *testing.T) {
	req := model.CleanupFinishRequest{ID: "c", CWD: "/x", Merged: true}
	got := WithCleanupFinishEvidence(req, "body "+model.IssueBodyCompletionStartMarker, " Closed ", model.CleanupRemoteBranchArtifactHead{BaseRefName: "main"})
	if !got.CompletionReflected || !got.IssueClosed || got.MergedBaseBranch != "main" || got.ID != "c" || got.CWD != "/x" || !got.Merged {
		t.Fatalf("evidence = %+v", got)
	}
	got = WithCleanupFinishEvidence(req, "plain", "open", model.CleanupRemoteBranchArtifactHead{})
	if got.CompletionReflected || got.IssueClosed || got.MergedBaseBranch != "" {
		t.Fatalf("negative evidence = %+v", got)
	}

	result := model.CleanupFinishResult{
		OK: false, ID: "c", Missing: []string{"worktree_clean", "phase_done", "phase_done"}, WorktreePath: "/wt", Branch: "feat",
		WorkspaceProcesses: []model.CleanupWorkspaceProcess{{PID: 4, Command: "vim", StartedAt: "t"}},
	}
	status := CleanupStatusFromFinish(model.IssueOpsCleanupStatus{RemoteArtifactURL: "https://x/1"}, result)
	if status.Ready || !status.OK || !status.Merged || status.ID != "c" || status.WorktreePath != "/wt" || status.Branch != "feat" || status.RemoteArtifactURL != "https://x/1" {
		t.Fatalf("status = %+v", status)
	}
	if !slices.Equal(status.Missing, []string{"phase_done", "worktree_clean"}) {
		t.Fatalf("missing must be sorted and deduplicated: %v", status.Missing)
	}
	if !slices.Equal(status.Warnings, []string{"4:vim:t", "apply가 프로세스 1개를 종료합니다"}) {
		t.Fatalf("warnings = %v", status.Warnings)
	}
	if !strings.Contains(status.Choices[0], "차단 해소") {
		t.Fatalf("choices = %v", status.Choices)
	}

	ready := CleanupStatusFromFinish(model.IssueOpsCleanupStatus{}, model.CleanupFinishResult{OK: true})
	if !ready.Ready || len(ready.Missing) != 0 || !strings.Contains(ready.Choices[0], "정리 진행") {
		t.Fatalf("ready = %+v", ready)
	}
	// OK=true with leftover Missing must not be ready.
	contradictory := CleanupStatusFromFinish(model.IssueOpsCleanupStatus{}, model.CleanupFinishResult{OK: true, Missing: []string{"x"}})
	if contradictory.Ready {
		t.Fatalf("contradictory = %+v", contradictory)
	}
}

func TestCovCFStatusWarningsOnlyForProcesses(t *testing.T) {
	if got := CleanupStatusWarnings(model.CleanupFinishResult{}); len(got) != 0 {
		t.Fatalf("none = %v", got)
	}
	got := CleanupStatusWarnings(model.CleanupFinishResult{WorkspaceProcesses: []model.CleanupWorkspaceProcess{{PID: 4, Command: "vim", StartedAt: "t"}}})
	if !slices.Equal(got, []string{"4:vim:t", "apply가 프로세스 1개를 종료합니다"}) {
		t.Fatalf("processes only = %v", got)
	}
}

func TestCovCFRemoteArtifactMissingFieldsAndMetadata(t *testing.T) {
	if got := RemoteArtifactMissing(model.IssueOpsRecord{}); !slices.Equal(got, []string{"remote_artifact"}) {
		t.Fatalf("nil = %v", got)
	}
	blank := model.IssueOpsRecord{RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Provider: " ", Labels: []string{" ", "a\x00b"}, Assignees: nil}}
	want := []string{"remote_artifact_assignees", "remote_artifact_kind", "remote_artifact_labels", "remote_artifact_provider", "remote_artifact_url"}
	if got := RemoteArtifactMissing(blank); !slices.Equal(got, want) {
		t.Fatalf("blank = %v", got)
	}
	full := model.IssueOpsRecord{RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Provider: "g", Kind: "pr", URL: "u", Labels: []string{"l"}, Assignees: []string{"a"}}}
	if got := RemoteArtifactMissing(full); len(got) != 0 {
		t.Fatalf("full = %v", got)
	}
}

func TestCovCFStatusBuildChildCloseAndPhase(t *testing.T) {
	record := model.IssueOpsRecord{ID: "c", Phase: model.IssueOpsPhasePlan,
		IssueLinks: []model.IssueOpsIssueLink{{Type: "related"}, {Type: "child", CloseVerifiedAt: " "}}}
	status := BuildCleanupStatus(record, model.IssueOpsCleanupStatusRequest{Merged: true}, CleanupStatusObservation{})
	for _, slug := range []string{"pr_phase", "remote_artifact", "child_tasks_closed", "worktree_path"} {
		if !slices.Contains(status.Missing, slug) {
			t.Errorf("missing %q in %v", slug, status.Missing)
		}
	}
	if slices.Contains(status.Missing, "remote_artifact_merged") {
		t.Errorf("merged request must not report unmerged: %v", status.Missing)
	}
	record.IssueLinks = []model.IssueOpsIssueLink{{Type: "related"}, {Type: "child", CloseVerifiedAt: "t"}}
	status = BuildCleanupStatus(record, model.IssueOpsCleanupStatusRequest{}, CleanupStatusObservation{})
	if slices.Contains(status.Missing, "child_tasks_closed") || !slices.Contains(status.Missing, "remote_artifact_merged") {
		t.Fatalf("verified child, unmerged = %v", status.Missing)
	}
}

func TestCovCFStatusBuildWorktreeAndRemoteObservations(t *testing.T) {
	record := model.IssueOpsRecord{ID: "c", Phase: model.IssueOpsPhaseDone, WorktreePath: "/wt", Branch: "feat",
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Provider: "g", Kind: "pr", URL: "u", Labels: []string{"l"}, Assignees: []string{"a"}}}
	req := model.IssueOpsCleanupStatusRequest{Merged: true}

	status := BuildCleanupStatus(record, req, CleanupStatusObservation{})
	if !slices.Equal(status.Missing, []string{"worktree_exists"}) {
		t.Fatalf("absent worktree = %v", status.Missing)
	}

	clean := CleanupStatusObservation{WorktreeExists: true, Branch: "feat", Remote: "origin"}
	status = BuildCleanupStatus(record, req, clean)
	if !status.Ready || len(status.Missing) != 0 || status.RemoteArtifactURL != "u" {
		t.Fatalf("clean = %+v", status)
	}

	facts := clean
	facts.StatusCode, facts.StatusError, facts.Branch, facts.RemoteCode, facts.RemoteError = 128, " fatal: bad ", "other", 2, " net down "
	status = BuildCleanupStatus(record, req, facts)
	if !slices.Equal(status.Missing, []string{"branch_match", "remote_branch_check_failed", "worktree_git_status"}) || !slices.Equal(status.Warnings, []string{"fatal: bad", "net down"}) {
		t.Fatalf("failures = %+v", status)
	}

	facts = clean
	facts.StatusOutput, facts.RemoteOutput = " M file", "abc\trefs/heads/feat"
	status = BuildCleanupStatus(record, req, facts)
	if !slices.Equal(status.Missing, []string{"remote_branch_absent", "worktree_clean"}) {
		t.Fatalf("dirty+remote present = %v", status.Missing)
	}

	facts = clean
	facts.Branch, facts.Remote = " ", " "
	status = BuildCleanupStatus(record, req, facts)
	if !slices.Equal(status.Missing, []string{"branch", "remote_branch_check_unavailable"}) {
		t.Fatalf("detached, no remote = %v", status.Missing)
	}

	// Detached HEAD with a remote configured skips the remote branch probe.
	facts = clean
	facts.Branch, facts.RemoteCode = "", 9
	status = BuildCleanupStatus(record, req, facts)
	if !slices.Equal(status.Missing, []string{"branch"}) {
		t.Fatalf("detached with remote = %v", status.Missing)
	}
}

func TestCovCFStatusNeedsMergeReadback(t *testing.T) {
	complete := &model.IssueOpsRemoteArtifactVerification{Provider: "g", Kind: "pr", URL: "u", Labels: []string{"l"}, Assignees: []string{"a"}}
	done := model.IssueOpsRecord{Phase: model.IssueOpsPhaseDone, RemoteArtifact: complete}
	if !CleanupStatusNeedsMergeReadback(done, true) {
		t.Fatal("done+complete+requested must read back")
	}
	if CleanupStatusNeedsMergeReadback(done, false) {
		t.Fatal("not requested")
	}
	notDone := done
	notDone.Phase = model.IssueOpsPhasePR
	if CleanupStatusNeedsMergeReadback(notDone, true) {
		t.Fatal("non-done phase")
	}
	if CleanupStatusNeedsMergeReadback(model.IssueOpsRecord{Phase: model.IssueOpsPhaseDone}, true) {
		t.Fatal("missing artifact")
	}
}

func TestCovCFDescribeAndRemainingProcesses(t *testing.T) {
	if got := DescribeCleanupProcesses(nil); got != "(none)" {
		t.Fatalf("none = %q", got)
	}
	procs := []model.CleanupWorkspaceProcess{{PID: 3, Command: "a"}, {PID: 5, Command: "b"}}
	if got := DescribeCleanupProcesses(procs); got != "3:a, 5:b" {
		t.Fatalf("describe = %q", got)
	}
	targets := []model.CleanupWorkspaceProcess{
		{PID: 1, StartedAt: "t1", Executable: "/a"},
		{PID: 2, StartedAt: "t2", Executable: "/b"},
		{PID: 3, StartedAt: "t3", Executable: "/c"},
	}
	occupants := []model.CleanupWorkspaceProcess{
		{PID: 1, StartedAt: "t1", Executable: "/a"},     // still running
		{PID: 2, StartedAt: "tX", Executable: "/b"},     // pid reused
		{PID: 9, StartedAt: "t9", Executable: "/other"}, // unrelated
	}
	got := RemainingCleanupProcesses(targets, occupants)
	if len(got) != 1 || got[0].PID != 1 {
		t.Fatalf("remaining = %+v", got)
	}
	if got := RemainingCleanupProcesses(targets, nil); len(got) != 0 {
		t.Fatalf("none remain = %+v", got)
	}
}

func TestCovCFRequesterOccupancyMissing(t *testing.T) {
	ancestry := map[int][]int{10: {5, 1}}
	if got, ok := CleanupRequesterOccupancyMissing(nil, ancestry, 99); ok || got != nil {
		t.Fatalf("unobservable = %v,%v", got, ok)
	}
	got, ok := CleanupRequesterOccupancyMissing([]model.CleanupWorkspaceProcess{{PID: 77}}, ancestry, 10)
	if !ok || got == nil || len(got) != 0 {
		t.Fatalf("unrelated occupant = %v,%v", got, ok)
	}
	for _, pid := range []int{10, 5, 1} {
		got, ok := CleanupRequesterOccupancyMissing([]model.CleanupWorkspaceProcess{{PID: 77}, {PID: pid}}, ancestry, 10)
		if !ok || !slices.Equal(got, []string{"requester_occupies_worktree"}) {
			t.Errorf("pid %d = %v,%v", pid, got, ok)
		}
	}
}

func TestCovCFOccupantReceiptsSortedByPID(t *testing.T) {
	got := CleanupOccupantReceipts([]model.CleanupWorkspaceProcess{
		{PID: 9, StartedAt: "t9", Executable: "/z", Command: "ignored"},
		{PID: 2, StartedAt: "t2", Executable: "/a"},
	})
	want := []model.NativeProcessReceipt{{PID: 2, StartedAt: "t2", Executable: "/a"}, {PID: 9, StartedAt: "t9", Executable: "/z"}}
	if !slices.Equal(got, want) {
		t.Fatalf("receipts = %+v", got)
	}
	if got := CleanupOccupantReceipts(nil); got == nil || len(got) != 0 {
		t.Fatalf("empty = %#v", got)
	}
}
