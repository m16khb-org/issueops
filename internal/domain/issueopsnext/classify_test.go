package issueopsnext

import (
	"slices"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
)

func defaultCompletion(phase issueopsnextcontract.Phase) Readiness {
	switch phase {
	case issueopsnextcontract.PhaseProblem:
		return Readiness{Missing: []string{"intent_contract"}}
	case issueopsnextcontract.PhaseGrill:
		return Readiness{Missing: []string{"branch", "issue_url"}}
	case issueopsnextcontract.PhaseImplement:
		return Readiness{Missing: []string{"devils_advocate_review"}}
	default:
		return Readiness{Ready: true}
	}
}

func baseRecord(phase issueopsnextcontract.Phase) Input {
	return Input{
		Record: issueopsnextcontract.Record{
			ID: "io-test", Repo: "/repo", Branch: "12-test", Phase: phase,
		},
		Completion: defaultCompletion,
		SourceRoot: "/repo",
		ActorHost:  "claude", ActorSessionID: "s-1",
	}
}

func withIssue(in Input) Input {
	in.Record.IssueURL = "https://github.com/acme/repo/issues/12"
	return withoutCompletionKey(in, issueopsnextcontract.PhaseGrill, "issue_url")
}

func withBranch(in Input) Input {
	in.Record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: in.Record.IssueURL, Branch: in.Record.Branch,
		BaseBranch: "main", BaseSHA: "a1b2c3d4", LinkVerified: true,
	}
	return withoutCompletionKey(in, issueopsnextcontract.PhaseGrill, "branch")
}

func withStagedPlan(in Input) Input {
	in.StagedPlan = true
	return in
}

func withStagedArchitecture(in Input) Input {
	in.StagedArchitecture = true
	return in
}

// withArchitecture는 아키텍처 문서를 스테이징하고 그 문서를 승인한 상태다.
func withArchitecture(in Input) Input {
	in.Record.ArchitectureReview = &reviewcontract.ArchitectureReview{
		ProblemSummary: "summary", ProposedArchitecture: "architecture",
		Verification: []string{"architecture review checked alternatives and risks"},
		Approved:     true, ReviewerContext: "subagent", ReviewedArchitectureDigest: "digest", ReviewedAt: "2026-09-05T00:00:00Z",
	}
	return withStagedArchitecture(in)
}

func withDA(in Input) Input {
	in.Record.DevilsAdvocateReview = &issueopscontract.IssueOpsDevilsAdvocateReview{
		Verdict: "pass", ReviewedPlanDigest: "digest", RecordedAt: "2026-09-05T00:01:00Z",
	}
	return withoutCompletionKey(in, issueopsnextcontract.PhaseImplement, "devils_advocate_review")
}

func withDAStop(in Input) Input {
	in.Record.DevilsAdvocateReview = &issueopscontract.IssueOpsDevilsAdvocateReview{
		Verdict: "stop", Findings: []string{"finding"}, Waived: false,
		ReviewedPlanDigest: "digest", RecordedAt: "2026-09-05T00:01:00Z",
	}
	return in
}

func withExecution(in Input, lease issueopscontract.WriteLease) Input {
	in.Record.Execution = &issueopscontract.Execution{
		Workspace: issueopscontract.Workspace{
			SourceRoot: "/repo", Root: "/repo.worktrees/12-test", Branch: in.Record.Branch,
		},
		Lease: lease,
	}
	in.WorktreePresent = true
	return in
}

func withSelfHolder(in Input) Input {
	return withExecution(in, issueopscontract.WriteLease{
		Generation: 1, Status: issueopsnextcontract.LeaseStatusActive,
		Holder: &issueopscontract.NativeActor{Host: "claude", SessionID: "s-1"},
	})
}

func withHolder(in Input, host, session string, alive bool) Input {
	in = withExecution(in, issueopscontract.WriteLease{
		Generation: 1, Status: issueopsnextcontract.LeaseStatusActive,
		Holder: &issueopscontract.NativeActor{Host: host, SessionID: session},
	})
	in.HolderLive = &alive
	return in
}

func withReleasedLease(in Input) Input {
	return withExecution(in, issueopscontract.WriteLease{
		Generation: 1, Status: issueopsnextcontract.LeaseStatusReleased,
	})
}

func withClaimableLease(in Input) Input {
	return withExecution(in, issueopscontract.WriteLease{
		Generation: 1, Status: issueopsnextcontract.LeaseStatusClaimable,
	})
}

func withPending(in Input) Input {
	in = withSelfHolder(in)
	in.Record.Execution.Pending = &issueopscontract.ExternalIntent{
		OperationID: "op-1", Kind: "create_issue", Marker: "marker", StartedAt: "2026-09-05T00:02:00Z",
	}
	return in
}

func withArtifact(in Input) Input {
	in.Record.RemoteArtifact = &issueopscontract.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/12",
		VerifiedAt: "2026-09-05T00:03:00Z",
	}
	return in
}

func withCompletionMissing(in Input, phase issueopsnextcontract.Phase, keys ...string) Input {
	inner := in.Completion
	in.Completion = func(candidate issueopsnextcontract.Phase) Readiness {
		if candidate == phase {
			return Readiness{Ready: len(keys) == 0, Missing: keys}
		}
		return inner(candidate)
	}
	return in
}

func withoutCompletionKey(in Input, phase issueopsnextcontract.Phase, key string) Input {
	inner := in.Completion
	in.Completion = func(candidate issueopsnextcontract.Phase) Readiness {
		readiness := inner(candidate)
		if candidate != phase {
			return readiness
		}
		var kept []string
		for _, item := range readiness.Missing {
			if item != key {
				kept = append(kept, item)
			}
		}
		readiness.Missing = kept
		readiness.Ready = len(kept) == 0
		return readiness
	}
	return in
}

func withLocal(in Input, keys ...string) Input {
	in.Local = &Readiness{Ready: len(keys) == 0, Missing: keys}
	return in
}

func TestClassifyStages(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		key  string
		idx  int
	}{
		{"no record", Input{}, "none", 0},
		{"pending intent blocks", withPending(baseRecord(issueopsnextcontract.PhaseImplement)), "blocked.pending", 4},
		{"other live holder", withHolder(baseRecord(issueopsnextcontract.PhaseImplement), "codex", "s-2", true), "blocked.holder_live", 4},
		{"stale holder takeover", withHolder(baseRecord(issueopsnextcontract.PhaseImplement), "codex", "s-2", false), "takeover", 4},
		{"grill without issue", baseRecord(issueopsnextcontract.PhaseGrill), "issue", 1},
		{"grill with issue no branch", withIssue(baseRecord(issueopsnextcontract.PhaseGrill)), "prepare", 2},
		{"plan right after link-issue auto-advance", withIssue(baseRecord(issueopsnextcontract.PhasePlan)), "prepare", 2},
		{"implement without execution", withIssue(baseRecord(issueopsnextcontract.PhaseImplement)), "prepare", 2},
		{"branch ready, nothing staged", withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan))), "plan.architecture", 3},
		{"architecture approved, no staged plan", withArchitecture(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))), "plan.write", 3},
		{"staged plan, architecture missing", withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))), "plan.architecture", 3},
		{"architecture approved, DA missing", withArchitecture(withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan))))), "plan.review", 3},
		{"DA stop unwaived", withDAStop(withArchitecture(withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))))), "plan.review", 3},
		{"all planner gates recorded", withDA(withArchitecture(withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))))), "plan.handoff", 3},
		{"claimable right after prepare", withClaimableLease(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))), "claim", 3},
		{"released lease in implement", withReleasedLease(withBranch(withIssue(baseRecord(issueopsnextcontract.PhaseImplement)))), "claim", 4},
		{"self holder still in plan phase", withSelfHolder(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))), "implement.enter", 4},
		{"self holder in implement", withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement)), "implement", 4},
		{"cleanup not recorded", withCompletionMissing(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), issueopsnextcontract.PhaseAISlopClean, "ai_slop_clean_at"), "clean", 5},
		{"cleanup recorded docs review missing", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), "project_docs_review"), "docs", 6},
		{"docs reviewed implementation review missing", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), "implementation_review"), "verify", 7},
		{"feedback contract update missing", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseFeedback)), "contract_feedback_issue_update"), "verify", 7},
		{"stale seal returns to clean", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), "ai_slop_clean_stale"), "clean", 5},
		{"gates ledger incomplete", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), "gates_incomplete:.issueops/gates.md"), "verify", 7},
		{"only upstream missing", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), "upstream"), "commit-push", 8},
		{"pushed and clean", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean))), "commit-push", 8},
		{"unmatched local key falls back", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseFeedback)), "branch_match"), "unknown", 7},
		{"pr without artifact", withSelfHolder(baseRecord(issueopsnextcontract.PhasePR)), "pr.create", 9},
		{"pr with artifact", withArtifact(withSelfHolder(baseRecord(issueopsnextcontract.PhasePR))), "pr.complete", 9},
		{"pr without execution", withIssue(baseRecord(issueopsnextcontract.PhasePR)), "unknown", 9},
		{"done", baseRecord(issueopsnextcontract.PhaseDone), "done", 10},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Classify(testCase.in)
			if got.Stage.Key != testCase.key || got.Stage.Index != testCase.idx {
				t.Fatalf("got %s/%d want %s/%d", got.Stage.Key, got.Stage.Index, testCase.key, testCase.idx)
			}
		})
	}
}

// 규칙 순서가 뒤집히면 done 사이클이 미해결 pending을 숨긴다. 규칙 2가 규칙
// 21보다 먼저여야 한다.
func TestClassifyChecksPendingBeforePhase(t *testing.T) {
	got := Classify(withPending(baseRecord(issueopsnextcontract.PhaseDone)))
	if got.Stage.Key != "blocked.pending" {
		t.Fatalf("pending intent must win over the done phase, got %s", got.Stage.Key)
	}
	if !strings.Contains(got.NextCommand, "execution reconcile") {
		t.Fatalf("pending intent must route to reconcile, got %q", got.NextCommand)
	}
}

func TestClassifyNextCommandTable(t *testing.T) {
	cases := []struct {
		name   string
		in     Input
		prefix string
		kind   string
	}{
		{"none starts a cycle", Input{SourceRoot: "/repo"}, "issueops start --repo /repo", "exact"},
		{"issue records intent", baseRecord(issueopsnextcontract.PhaseGrill), "issueops intent record", "template"},
		{"handoff prepares execution", withDA(withArchitecture(withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))))), "issueops execution prepare", "template"},
		{"stop verdict regresses", withDAStop(withArchitecture(withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))))), "issueops regress --id io-test", "template"},
		{"commit-push checks strict readiness", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), "upstream"), "issueops pr-readiness --id io-test --strict", "exact"},
		{"done checks cleanup", baseRecord(issueopsnextcontract.PhaseDone), "issueops cleanup status --id io-test --merged", "exact"},
		{"unknown falls back to status", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseFeedback)), "branch_match"), "issueops status --id io-test", "exact"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Classify(testCase.in)
			if !strings.HasPrefix(got.NextCommand, testCase.prefix) {
				t.Fatalf("next command %q must start with %q", got.NextCommand, testCase.prefix)
			}
			if got.NextCommandKind != testCase.kind {
				t.Fatalf("next command kind %q want %q", got.NextCommandKind, testCase.kind)
			}
		})
	}
}

func TestClassifyHandoffRendersThePrepareCommandWithoutRunSelection(t *testing.T) {
	got := Classify(withDA(withArchitecture(withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))))))
	if want := "issueops execution prepare --id io-test $ACTOR_FLAGS --json"; got.NextCommand != want {
		t.Fatalf("next command %q want %q", got.NextCommand, want)
	}
}

// 1단계는 missing을 알파벳 순으로 쓰면 안 된다. branch는 2단계 항목이다.
func TestClassifyIssueStageNeverRecommendsBranchPrepare(t *testing.T) {
	got := Classify(baseRecord(issueopsnextcontract.PhaseGrill))
	if strings.Contains(got.NextCommand, "branch prepare") {
		t.Fatalf("stage 1 must not recommend branch prepare, got %q", got.NextCommand)
	}
	if len(got.Missing) == 0 || got.Missing[0] != "intent_contract" {
		t.Fatalf("stage 1 missing must lead with intent_contract, got %v", got.Missing)
	}
}

func TestClassifyExits(t *testing.T) {
	self := Classify(withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement)))
	if !strings.HasPrefix(self.Exits.PauseCommand, "issueops execution release") {
		t.Fatalf("self holder must be able to pause, got %q", self.Exits.PauseCommand)
	}
	stale := Classify(withHolder(baseRecord(issueopsnextcontract.PhaseImplement), "codex", "s-2", false))
	if !strings.HasPrefix(stale.Exits.TakeoverCommand, "issueops execution replace") {
		t.Fatalf("stale holder must be replaceable, got %q", stale.Exits.TakeoverCommand)
	}
	if !strings.HasPrefix(self.Exits.AbandonCommand, "issueops cleanup abandon --id io-test --reason") {
		t.Fatalf("abandon must always be offered, got %q", self.Exits.AbandonCommand)
	}
	none := Classify(Input{})
	if none.Exits.AbandonCommand != "" {
		t.Fatalf("without a cycle there is nothing to abandon, got %q", none.Exits.AbandonCommand)
	}
}

// 관측하지 못한 홀더는 살아 있는 것으로 본다. 확인하지 않은 세션의 lease를
// 빼앗으라고 권하면 두 세션이 같은 워크트리를 쓴다.
func TestClassifyTreatsUnobservedHolderAsLive(t *testing.T) {
	in := withHolder(baseRecord(issueopsnextcontract.PhaseImplement), "codex", "s-2", true)
	in.HolderLive = nil
	if got := Classify(in); got.Stage.Key != "blocked.holder_live" {
		t.Fatalf("an unobserved holder must block, got %s", got.Stage.Key)
	}
}

func TestOwnerCommandCoversGateMap(t *testing.T) {
	keys := []string{
		"intent_contract", "raw_request", "interpreted_intent", "success_criteria",
		"plan_prep_decisions", "plan_prep_related_issues", "plan_prep_web_research", "plan_prep_codebase_survey",
		"issue_url", "branch", "branch_prepare", "split_decision", "domain_review",
		"architecture_review", "architecture_open_questions", "compatibility_review", "compatibility_blockers",
		"devils_advocate_review", "devils_advocate_review_stale",
		"worktree_path", "worktree_exists", "plan_artifact", "plan_path", "plan_exists", "plan_in_worktree",
		"execution", "execution_valid", "execution_worktree_match", "execution_write_lease",
		"implementation_changes", "ai_slop_clean_at", "ai_slop_clean_head", "ai_slop_clean_fingerprint",
		"ai_slop_clean_stale", "cleanup_evidence", "verification_evidence", "current_fingerprint",
		"implementation_review", "implementation_review_stale",
		"project_docs_review", "project_docs_review_stale",
		"schema_evidence", "schema_evidence_stale",
		"feedback_classification", "feedback_resolution", "contract_feedback_issue_update",
		"remote_artifact", "child_incomplete", "child_unvalidated", "child_rejected_unresolved",
		"gates_incomplete:.issueops/gates.md",
		"worktree_clean", "upstream", "upstream_fetch", "upstream_synced", "branch_match",
	}
	for _, key := range keys {
		command := OwnerCommand("io-test", key)
		if strings.TrimSpace(command) == "" {
			t.Fatalf("missing owner command for %q", key)
		}
		if !strings.HasPrefix(command, "issueops ") {
			t.Fatalf("owner command for %q must be an issueops command, got %q", key, command)
		}
	}
	if got := OwnerCommand("io-test", "who_knows"); got != "issueops status --id io-test --json" {
		t.Fatalf("an unknown gate must fall back to status, got %q", got)
	}
}

// Record commands require RECORD_ACTOR_FLAGS (the holder check refuses a
// mutation without them), so their templates carry the placeholder and are
// never reported as exact commands.
func TestRecordOwnerCommandsCarryActorFlagPlaceholder(t *testing.T) {
	for _, key := range []string{
		"implementation_review", "project_docs_review", "schema_evidence",
		"feedback_classification", "feedback_resolution", "contract_feedback_issue_update", "remote_artifact",
	} {
		command := OwnerCommand("io-test", key)
		if !strings.HasSuffix(command, " $RECORD_ACTOR_FLAGS") {
			t.Errorf("%s: %q lacks $RECORD_ACTOR_FLAGS", key, command)
		}
		if kind := finish(Input{}, Decision{NextCommand: command}).NextCommandKind; kind != issueopsnextcontract.NextCommandKindTemplate {
			t.Errorf("%s: kind %q, want template", key, kind)
		}
	}
}

func TestClassifyRootConflictBlocksOnlyExecutionlessCyclesFromPlanOnward(t *testing.T) {
	const blockedRoot = "blocked.root_conflict"
	cases := []struct {
		name string
		in   Input
		want bool
	}{
		{"plan without execution", withIssue(baseRecord(issueopsnextcontract.PhasePlan)), true},
		{"implement without execution", withIssue(baseRecord(issueopsnextcontract.PhaseImplement)), true},
		{"grill is earlier than plan", withIssue(baseRecord(issueopsnextcontract.PhaseGrill)), false},
		{"problem is earlier than plan", baseRecord(issueopsnextcontract.PhaseProblem), false},
		{"execution already owns the root", withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement)), false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			in := testCase.in
			in.RootConflictID = " io-other "
			got := Classify(in)
			if (got.Stage.Key == blockedRoot) != testCase.want {
				t.Fatalf("stage %s, blocked.root_conflict want %v", got.Stage.Key, testCase.want)
			}
			if !testCase.want {
				return
			}
			if got.Stage.Index != 3 {
				t.Fatalf("root conflict index %d want 3", got.Stage.Index)
			}
			if got.NextCommand != "issueops list --repo /repo --json" {
				t.Fatalf("next command %q", got.NextCommand)
			}
			if len(got.Warnings) != 1 || got.Warnings[0] != "canonical worktree root is already claimed by cycle io-other" {
				t.Fatalf("warnings %v", got.Warnings)
			}
		})
	}
	blank := withIssue(baseRecord(issueopsnextcontract.PhasePlan))
	blank.RootConflictID = "  "
	if got := Classify(blank); got.Stage.Key == blockedRoot {
		t.Fatal("a blank root conflict id must not block")
	}
}

func TestClassifyUnknownReportsUnmatchedLocalGates(t *testing.T) {
	got := Classify(withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseFeedback)), "branch_match", "other"))
	if got.Stage.Key != "unknown" {
		t.Fatalf("stage %s want unknown", got.Stage.Key)
	}
	if strings.Join(got.Missing, ",") != "branch_match,other" {
		t.Fatalf("missing %v", got.Missing)
	}
	if len(got.Warnings) != 1 || got.Warnings[0] != "unmatched readiness gates: branch_match, other" {
		t.Fatalf("warnings %v", got.Warnings)
	}

	for name, in := range map[string]Input{
		"no local readiness":    withIssue(baseRecord(issueopsnextcontract.PhasePR)),
		"empty local readiness": withLocal(withIssue(baseRecord(issueopsnextcontract.PhasePR))),
	} {
		got := Classify(in)
		if got.Stage.Key != "unknown" {
			t.Fatalf("%s: stage %s want unknown", name, got.Stage.Key)
		}
		if len(got.Missing) != 0 || len(got.Warnings) != 0 {
			t.Fatalf("%s: unknown without gates must not report missing %v / warnings %v", name, got.Missing, got.Warnings)
		}
	}
}

func TestClassifyMissingIssueURLOnlyClassifiesProblemAndGrill(t *testing.T) {
	for _, phase := range []issueopsnextcontract.Phase{issueopsnextcontract.PhaseProblem, issueopsnextcontract.PhaseGrill} {
		got := Classify(baseRecord(phase))
		if got.Stage.Key != "issue" || got.Stage.Index != 1 {
			t.Fatalf("%s without issue: got %s/%d want issue/1", phase, got.Stage.Key, got.Stage.Index)
		}
	}
	got := Classify(baseRecord(issueopsnextcontract.PhasePlan))
	if got.Stage.Key != "unknown" {
		t.Fatalf("plan without issue url must not be treated as stage 1, got %s", got.Stage.Key)
	}
}

func TestClassifyBranchPrepareCommandDependsOnPhase(t *testing.T) {
	problem := Classify(withIssue(baseRecord(issueopsnextcontract.PhaseProblem)))
	if problem.Stage.Key != "prepare" || problem.NextCommand != "issueops phase --id io-test --to grill --json" {
		t.Fatalf("problem with issue: got %s %q", problem.Stage.Key, problem.NextCommand)
	}
	if len(problem.Missing) != 1 || problem.Missing[0] != "intent_contract" {
		t.Fatalf("problem with issue must surface stage-one gates, got %v", problem.Missing)
	}
	grill := Classify(withIssue(baseRecord(issueopsnextcontract.PhaseGrill)))
	if grill.Stage.Key != "prepare" || strings.Contains(grill.NextCommand, "--to grill") {
		t.Fatalf("grill with issue must not re-advance to grill, got %s %q", grill.Stage.Key, grill.NextCommand)
	}
	if grill.NextCommand != OwnerCommand("io-test", "branch_prepare") {
		t.Fatalf("grill next command %q want owner command for branch_prepare", grill.NextCommand)
	}
	if len(grill.Missing) != 1 || grill.Missing[0] != "branch_prepare" {
		t.Fatalf("grill missing %v", grill.Missing)
	}
}

func TestClassifyRevokingLeaseOffersTakeover(t *testing.T) {
	revoking := withExecution(baseRecord(issueopsnextcontract.PhaseImplement), issueopscontract.WriteLease{
		Generation: 4, Status: issueopsnextcontract.LeaseStatusRevoking,
	})
	got := Classify(revoking)
	if got.Stage.Key != "takeover" {
		t.Fatalf("revoking lease stage %s want takeover", got.Stage.Key)
	}
	const replace = "issueops execution replace --id io-test --expected-generation 4 --preview"
	if got.NextCommand != replace || got.Exits.TakeoverCommand != replace {
		t.Fatalf("next %q takeover exit %q want %q", got.NextCommand, got.Exits.TakeoverCommand, replace)
	}
	if got.Exits.PauseCommand != "" {
		t.Fatalf("a revoking lease cannot be paused, got %q", got.Exits.PauseCommand)
	}

	self := Classify(withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement)))
	if self.Exits.TakeoverCommand != "" {
		t.Fatalf("an active self holder must not be offered takeover, got %q", self.Exits.TakeoverCommand)
	}
	live := Classify(withHolder(baseRecord(issueopsnextcontract.PhaseImplement), "codex", "s-2", true))
	if live.Exits.TakeoverCommand != "" {
		t.Fatalf("a live foreign holder must not be offered takeover, got %q", live.Exits.TakeoverCommand)
	}
}

func TestClassifyTakeoverPrefersWriterlessRecovery(t *testing.T) {
	in := withHolder(baseRecord(issueopsnextcontract.PhaseImplement), "codex", "s-2", false)
	in.WriterlessRecovery = "  issueops execution recover --id io-test  "
	if got := Classify(in); got.NextCommand != "issueops execution recover --id io-test" {
		t.Fatalf("writerless recovery must win, got %q", got.NextCommand)
	}
	in.WriterlessRecovery = "   "
	if got := Classify(in); got.NextCommand != "issueops execution replace --id io-test --expected-generation 1 --preview" {
		t.Fatalf("blank recovery must fall back to replace, got %q", got.NextCommand)
	}
}

func TestClassifyRendersGenerationInTakeoverCommands(t *testing.T) {
	revoking := func(generation uint64) Input {
		return withExecution(baseRecord(issueopsnextcontract.PhaseImplement), issueopscontract.WriteLease{
			Generation: generation, Status: issueopsnextcontract.LeaseStatusRevoking,
		})
	}
	if got := Classify(revoking(12)); !strings.Contains(got.NextCommand, "--expected-generation 12 ") ||
		!strings.Contains(got.Exits.TakeoverCommand, "--expected-generation 12 ") {
		t.Fatalf("generation 12 must render, got next %q exit %q", got.NextCommand, got.Exits.TakeoverCommand)
	}
	if got := Classify(revoking(0)); !strings.Contains(got.NextCommand, "--expected-generation <N> ") ||
		!strings.Contains(got.Exits.TakeoverCommand, "--expected-generation <N> ") {
		t.Fatalf("unknown generation must render a placeholder, got next %q exit %q", got.NextCommand, got.Exits.TakeoverCommand)
	}
}

func TestClassifyIssueStageFallsBackToIntentContractWithoutMissingGates(t *testing.T) {
	in := baseRecord(issueopsnextcontract.PhaseGrill)
	in.Completion = func(issueopsnextcontract.Phase) Readiness { return Readiness{Ready: true} }
	got := Classify(in)
	if got.Stage.Key != "issue" {
		t.Fatalf("stage %s want issue", got.Stage.Key)
	}
	if got.NextCommand != OwnerCommand("io-test", "intent_contract") {
		t.Fatalf("next command %q want owner command for intent_contract", got.NextCommand)
	}
}

func TestClassifyWarningsPassThroughLocalAndWorktreeObservations(t *testing.T) {
	const missingRoot = "canonical worktree is missing at /repo.worktrees/12-test"
	withHead := func(finalHead, observed string) Input {
		in := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
		in.Record.Execution.Completion = &issueopscontract.ExecutionCompletion{FinalHead: finalHead}
		in.WorktreeHead = observed
		return in
	}
	noCompletion := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
	noCompletion.WorktreeHead = "def"
	missing := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
	missing.WorktreePresent = false
	cases := []struct {
		name string
		in   Input
		want []string
	}{
		{"worktree missing", missing, []string{missingRoot}},
		{"worktree present", withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement)), nil},
		{"head differs", withHead("abc", "def"), []string{"worktree head def differs from the recorded completion head abc"}},
		{"head differs after trimming", withHead(" abc ", " def "), []string{"worktree head def differs from the recorded completion head abc"}},
		{"head matches after trimming", withHead(" abc ", "abc "), nil},
		{"observed head blank", withHead("abc", "  "), nil},
		{"recorded head blank", withHead("  ", "def"), nil},
		{"no completion recorded", noCompletion, nil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Classify(testCase.in).Warnings
			if strings.Join(got, "|") != strings.Join(testCase.want, "|") {
				t.Fatalf("warnings %q want %q", got, testCase.want)
			}
		})
	}

	local := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
	local.Local = &Readiness{Warnings: []string{"base drift"}}
	if got := Classify(local).Warnings; len(got) != 1 || got[0] != "base drift" {
		t.Fatalf("local warnings must pass through, got %q", got)
	}
	local.WorktreePresent = false
	if got := Classify(local).Warnings; len(got) != 2 || got[0] != "base drift" || got[1] != missingRoot {
		t.Fatalf("local warnings must precede worktree warnings, got %q", got)
	}
	blankRoot := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
	blankRoot.Record.Execution.Workspace.Root = "  "
	blankRoot.WorktreePresent = false
	if got := Classify(blankRoot).Warnings; len(got) != 0 {
		t.Fatalf("a blank workspace root has no worktree to miss, got %q", got)
	}
}

func TestClassifyPlanStagesArchitectureFirst(t *testing.T) {
	plan := withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))
	stagedUnreviewed := withStagedArchitecture(plan)
	for _, step := range []struct {
		name    string
		in      Input
		stage   string
		missing string
		command string
	}{
		{"nothing staged", plan, "plan.architecture", "architecture_artifact", "issueops artifact stage --id io-test --name architecture"},
		{"plan staged before architecture", withStagedPlan(plan), "plan.architecture", "architecture_artifact", "issueops artifact stage --id io-test --name architecture"},
		{"architecture staged, not reviewed", stagedUnreviewed, "plan.architecture", "architecture_review", "issueops architecture review --id io-test"},
		{"architecture approved", withArchitecture(plan), "plan.write", "plan_artifact", "issueops artifact stage --id io-test --name plan"},
		{"plan staged after architecture", withStagedPlan(withArchitecture(plan)), "plan.review", "devils_advocate_review", "issueops devils-advocate review --id io-test"},
		{"devils advocate recorded", withDA(withStagedPlan(withArchitecture(plan))), "plan.handoff", "", "issueops execution prepare --id io-test"},
	} {
		t.Run(step.name, func(t *testing.T) {
			got := Classify(step.in)
			if got.Stage.Key != step.stage || !strings.HasPrefix(got.NextCommand, step.command) {
				t.Fatalf("stage=%q command=%q, want %q starting with %q", got.Stage.Key, got.NextCommand, step.stage, step.command)
			}
			if step.missing == "" && len(got.Missing) != 0 || step.missing != "" && !slices.Equal(got.Missing, []string{step.missing}) {
				t.Fatalf("missing=%v, want %q", got.Missing, step.missing)
			}
		})
	}
}

func TestClassifyArchitectureReviewStaleStaysInArchitectureStage(t *testing.T) {
	in := withDA(withStagedPlan(withArchitecture(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan))))))
	if got := Classify(in); got.Stage.Key != "plan.handoff" {
		t.Fatalf("fresh approval must reach handoff, got %q", got.Stage.Key)
	}
	in.ArchitectureReviewStale = true
	got := Classify(in)
	if got.Stage.Key != "plan.architecture" || !slices.Equal(got.Missing, []string{"architecture_review_stale"}) {
		t.Fatalf("stale approval must stay in plan.architecture: stage=%q missing=%v", got.Stage.Key, got.Missing)
	}
	if !strings.HasPrefix(got.NextCommand, "issueops architecture review --id io-test") {
		t.Fatalf("stale approval is re-recorded with architecture review, got %q", got.NextCommand)
	}
}

func TestClassifyDelegatedChildSkipsArchitectureStage(t *testing.T) {
	child := withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan))))
	child.Record.ArchitectureReview = &reviewcontract.ArchitectureReview{
		ProblemSummary: "summary", ProposedArchitecture: "architecture",
		Verification: []string{"architecture review checked alternatives and risks"},
		Approved:     true, ReviewerContext: issueopsdomain.ParentReviewPattern, ReviewedAt: "2026-09-05T00:00:00Z",
	}
	child.ArchitectureReviewStale = true
	if got := Classify(child); got.Stage.Key != "plan.review" {
		t.Fatalf("inherited approval must skip the architecture stage, got %q missing=%v", got.Stage.Key, got.Missing)
	}
	child.Record.ArchitectureReview.ReviewerContext = "subagent"
	if got := Classify(child); got.Stage.Key != "plan.architecture" {
		t.Fatalf("a reviewed child without its own staged architecture stays in plan.architecture, got %q", got.Stage.Key)
	}
}
