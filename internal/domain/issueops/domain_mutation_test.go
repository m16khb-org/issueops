package issueops

import (
	"slices"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestValidateBranchAcceptsEveryDecimalDigitInIssueNumber(t *testing.T) {
	for _, branch := range []string{"9-x", "09-x", "90-x"} {
		if err := ValidateBranch(branch); err != nil {
			t.Errorf("%s: %v", branch, err)
		}
	}
	for _, branch := range []string{"1a-x", "1/-x", ":1-x"} {
		if err := ValidateBranch(branch); err == nil {
			t.Errorf("%s accepted", branch)
		}
	}
}

func TestChildDroppedReasonMinimumLengthIsInclusive(t *testing.T) {
	entry := model.IssueOpsChildStatusEntry{ValidationVerdict: "dropped", ValidationReason: "0123456789", ValidatedAt: "2026-01-01T00:00:00Z"}
	if !ChildDropped(entry) {
		t.Fatal("10-character reason must count as dropped")
	}
	entry.ValidationReason = "012345678"
	if ChildDropped(entry) {
		t.Fatal("9-character reason must not count as dropped")
	}
}

func TestPrepareChildVerdictReasonMinimumLengthIsInclusive(t *testing.T) {
	reason, _, err := PrepareChildVerdict("dropped", " 0123456789 ", nil)
	if err != nil || reason != "0123456789" {
		t.Fatalf("10 chars: reason=%q err=%v", reason, err)
	}
	if _, _, err := PrepareChildVerdict("dropped", "012345678", nil); err == nil || !strings.Contains(err.Error(), "at least 10 characters") {
		t.Fatalf("9 chars: %v", err)
	}
}

func TestBuildChildStatusDroppedReceiptAtMinimumReasonIsNotOrphaned(t *testing.T) {
	parent := model.IssueOpsRecord{ID: "parent", ChildCycles: []model.IssueOpsChildCycleRef{
		{CycleID: "edge", ValidationVerdict: "dropped", ValidationReason: "0123456789", ValidatedAt: "2026-01-01T00:00:00Z"},
		{CycleID: "short", ValidationVerdict: "dropped", ValidationReason: "012345678", ValidatedAt: "2026-01-01T00:00:00Z"},
	}}
	got := BuildChildStatus(parent, nil)
	if !slices.Equal(got.Orphaned, []string{"short"}) {
		t.Fatalf("orphaned=%v", got.Orphaned)
	}
}

func TestBuildChildStatusChildIssueURLPrefersParentRefOverDelegation(t *testing.T) {
	parent := model.IssueOpsRecord{ID: "parent", ChildCycles: []model.IssueOpsChildCycleRef{
		{CycleID: "kept", ChildIssueURL: "https://example.test/ref"},
		{CycleID: "filled"},
		{CycleID: "plain"},
	}}
	scanned := map[string]model.IssueOpsRecord{
		"kept":   {ID: "kept", Delegation: &model.IssueOpsDelegationContract{ChildIssueURL: " https://example.test/delegation-kept "}},
		"filled": {ID: "filled", Delegation: &model.IssueOpsDelegationContract{ChildIssueURL: " https://example.test/delegation-filled "}},
		"plain":  {ID: "plain"},
		"extra":  {ID: "extra", Delegation: &model.IssueOpsDelegationContract{ChildIssueURL: "https://example.test/extra"}},
	}
	got := map[string]string{}
	for _, entry := range BuildChildStatus(parent, scanned).Children {
		got[entry.CycleID] = entry.ChildIssueURL
	}
	want := map[string]string{
		"kept": "https://example.test/ref", "filled": "https://example.test/delegation-filled",
		"plain": "", "extra": "https://example.test/extra",
	}
	for id, url := range want {
		if got[id] != url {
			t.Errorf("%s: url=%q want %q", id, got[id], url)
		}
	}
}

func TestValidateClosedChildObservationNamesRemoteState(t *testing.T) {
	if err := ValidateClosedChildObservation("u", "CLOSED"); err != nil {
		t.Fatalf("closed: %v", err)
	}
	if err := ValidateClosedChildObservation("u1", ""); err == nil || !strings.Contains(err.Error(), "child u1 is unobserved remotely") {
		t.Fatalf("empty state: %v", err)
	}
	if err := ValidateClosedChildObservation("u2", "open"); err == nil || !strings.Contains(err.Error(), "child u2 is open remotely") {
		t.Fatalf("open state: %v", err)
	}
}

func TestCleanupFinishPreviewUnmergedNeedsRecordedSupersessionWithoutError(t *testing.T) {
	record := model.IssueOpsRecord{ID: "cycle", Phase: model.IssueOpsPhaseDone}
	missingMerged := func(inventory model.CleanupFinishInventory, observed CleanupFinishObservation) bool {
		result := BuildCleanupFinishPreview(record, model.CleanupFinishRequest{}, inventory, observed)
		return slices.Contains(result.Missing, "remote_artifact_merged")
	}
	if missingMerged(model.CleanupFinishInventory{SupersededBy: "next"}, CleanupFinishObservation{}) {
		t.Fatal("recorded supersession without an error must satisfy the merge gate")
	}
	if !missingMerged(model.CleanupFinishInventory{SupersededBy: "next"}, CleanupFinishObservation{SupersedeError: "lookup failed"}) {
		t.Fatal("supersession lookup error must keep the merge gate open")
	}
	if !missingMerged(model.CleanupFinishInventory{}, CleanupFinishObservation{}) {
		t.Fatal("no supersession must keep the merge gate open")
	}
	merged := BuildCleanupFinishPreview(record, model.CleanupFinishRequest{Merged: true}, model.CleanupFinishInventory{}, CleanupFinishObservation{SupersedeError: "x"})
	if slices.Contains(merged.Missing, "remote_artifact_merged") {
		t.Fatal("merged request must not require supersession")
	}
}

func TestBuildCleanupStatusPRPhaseSatisfiesPRRequirement(t *testing.T) {
	record := model.IssueOpsRecord{ID: "cycle", WorktreePath: "/worktree", Branch: "1-x", RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/1", Labels: []string{"ready"}, Assignees: []string{"owner"},
	}}
	req := model.IssueOpsCleanupStatusRequest{Merged: true}
	facts := CleanupStatusObservation{WorktreeExists: true, Branch: "1-x", Remote: "origin"}
	record.Phase = model.IssueOpsPhasePR
	if got := BuildCleanupStatus(record, req, facts); slices.Contains(got.Missing, "pr_phase") {
		t.Fatalf("pr phase flagged: %v", got.Missing)
	}
	record.Phase = model.IssueOpsPhasePlan
	if got := BuildCleanupStatus(record, req, facts); !slices.Contains(got.Missing, "pr_phase") {
		t.Fatalf("earlier phase not flagged: %v", got.Missing)
	}
}

func TestExecutionReadinessMissingRequiresActiveLeaseWithHolder(t *testing.T) {
	exec := covExecDirect()
	record := model.IssueOpsRecord{Execution: &exec}
	if got := ExecutionReadinessMissing(record, true); len(got) != 0 {
		t.Fatalf("active lease with holder: %v", got)
	}
	noHolder := covExecDirect()
	noHolder.Lease.Holder = nil
	record.Execution = &noHolder
	if got := ExecutionReadinessMissing(record, true); !slices.Contains(got, "execution_write_lease") {
		t.Fatalf("active lease without holder: %v", got)
	}
	released := covExecDirect()
	released.Lease.Status = model.LeaseStatusReleased
	record.Execution = &released
	if got := ExecutionReadinessMissing(record, true); !slices.Contains(got, "execution_write_lease") {
		t.Fatalf("released lease with holder: %v", got)
	}
}

func TestRenderDevilsAdvocateSectionOmitsFindingCountWhenNone(t *testing.T) {
	got := RenderDevilsAdvocateSection(SectionInput{Rounds: []model.PlanReviewRound{
		{Verdict: "stop", Findings: 2}, {Verdict: "stop", Findings: 0},
	}})
	if strings.Contains(got, "지적 0건") || !strings.Contains(got, "(지적 2건)") {
		t.Fatalf("section: %s", got)
	}
}

func TestSectionBudgetAndMergeHandleMarkerAtBodyStart(t *testing.T) {
	const start, end = "<!--S-->", "<!--E-->"
	block := start + "old" + end
	body := block + "\ntail"
	if got, want := SectionBudget(body, 100, start, end), 100-len("\ntail"); got != want {
		t.Fatalf("budget=%d want %d", got, want)
	}
	if got, want := MergeManagedSection(body, start+"new"+end, start, end), start+"new"+end+"\ntail"; got != want {
		t.Fatalf("merge=%q want %q", got, want)
	}
}

func TestRenderSectionCompletionBudgetIsInclusive(t *testing.T) {
	req := SectionInput{Section: model.IssueBodySectionCompletion, Completion: &model.RemoteCompletionSection{ResultBody: "done"}}
	section := RenderCompletionSection(*req.Completion)
	got, _, _, err := RenderSection(req, len(section))
	if err != nil || got != section {
		t.Fatalf("section at limit: %q err=%v", got, err)
	}
	if _, _, _, err := RenderSection(req, len(section)-1); err == nil || !strings.Contains(err.Error(), "exceeds the body budget") {
		t.Fatalf("section over limit: %v", err)
	}
}

func TestValidatePhaseProgressionAllowsStayingInPhase(t *testing.T) {
	if err := ValidatePhaseProgression(model.IssueOpsPhasePR, model.IssueOpsPhasePR); err != nil {
		t.Fatalf("same phase: %v", err)
	}
	if err := ValidatePhaseProgression(model.IssueOpsPhasePR, model.IssueOpsPhasePlan); err == nil {
		t.Fatal("backward move accepted")
	}
}

func TestPRGitReadinessUpstreamSyncRequiresExactlyZeroZero(t *testing.T) {
	facts := PRGitFacts{RootAvailable: true, GitWorktree: true, FingerprintVerified: true, WorktreeClean: true, Upstream: "origin/x", SyncUpstream: true}
	for counts, wantMissing := range map[string]bool{"0 0": false, "1 0": true, "0 1": true, "0": true, "0 0 0": true, "": true} {
		facts.UpstreamCounts = counts
		missing, _ := PRGitReadiness(model.IssueOpsRecord{}, facts)
		if slices.Contains(missing, "upstream_synced") != wantMissing {
			t.Errorf("counts=%q missing=%v", counts, missing)
		}
	}
}

func TestReviewMetricsNamesOneBasedRoundOfUnparseableTimestamp(t *testing.T) {
	record := model.IssueOpsRecord{ID: "cycle", DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{
		Verdict: "pass", RecordedAt: "2026-01-01T00:00:10Z",
		History: []model.IssueOpsDevilsAdvocateRound{{Verdict: "revise", RecordedAt: "bad"}},
	}}
	_, warnings := ReviewMetricsForRecord(record)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "devil's-advocate round 1 has an unparseable recorded_at") {
		t.Fatalf("warnings=%v", warnings)
	}
}

// An inline plan review says so in the issue: readers otherwise take
// "1차 통과" for an independent review.
func TestRenderDevilsAdvocateSectionNamesAnInlineReview(t *testing.T) {
	inline := RenderDevilsAdvocateSection(SectionInput{Verdict: "pass", Findings: []string{"a"}, ReviewerContext: "inline"})
	if !strings.Contains(inline, "계획 검토: 1차 통과(의견 1건) (작성 세션이 직접 검토)") {
		t.Fatalf("inline section: %q", inline)
	}
	if sub := RenderDevilsAdvocateSection(SectionInput{Verdict: "pass", ReviewerContext: "subagent"}); strings.Contains(sub, "작성 세션") {
		t.Fatalf("subagent section marked inline: %q", sub)
	}
}
