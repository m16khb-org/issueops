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
		// A sync-base merge after the pr transition stales the seals; create-pr
		// would refuse, so next returns to the stage that reseals.
		{"pr with stale seal returns to clean", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhasePR)), "ai_slop_clean_stale", "implementation_review_stale"), "clean", 5},
		{"pr with ready readiness creates", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhasePR))), "pr.create", 9},
		{"pr with artifact", withArtifact(withSelfHolder(baseRecord(issueopsnextcontract.PhasePR))), "pr.complete", 9},
		{"pr without execution", withIssue(baseRecord(issueopsnextcontract.PhasePR)), "unknown", 9},
		{"done", baseRecord(issueopsnextcontract.PhaseDone), "done", 10},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := classifyTemplate(testCase.in)
			if got.Stage.Key != testCase.key || got.Stage.Index != testCase.idx {
				t.Fatalf("got %s/%d want %s/%d", got.Stage.Key, got.Stage.Index, testCase.key, testCase.idx)
			}
		})
	}
}

// 규칙 순서가 뒤집히면 done 사이클이 미해결 pending을 숨긴다. 규칙 2가 규칙
// 21보다 먼저여야 한다.
func TestClassifyChecksPendingBeforePhase(t *testing.T) {
	got := classifyTemplate(withPending(baseRecord(issueopsnextcontract.PhaseDone)))
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
		{"pushed and clean transitions to pr", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean))), "issueops phase --id io-test --to pr $RECORD_ACTOR_FLAGS", "template"},
		{"done previews the merge", baseRecord(issueopsnextcontract.PhaseDone), "issueops remote merge-pr --id io-test --json", "exact"},
		{"unknown falls back to status", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseFeedback)), "branch_match"), "issueops status --id io-test", "exact"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := classifyTemplate(testCase.in)
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
	got := classifyTemplate(withDA(withArchitecture(withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))))))
	if want := "issueops execution prepare --id io-test $ACTOR_FLAGS --json"; got.NextCommand != want {
		t.Fatalf("next command %q want %q", got.NextCommand, want)
	}
}

// 1단계는 missing을 알파벳 순으로 쓰면 안 된다. branch는 2단계 항목이다.
func TestClassifyIssueStageNeverRecommendsBranchPrepare(t *testing.T) {
	got := classifyTemplate(baseRecord(issueopsnextcontract.PhaseGrill))
	if strings.Contains(got.NextCommand, "branch prepare") {
		t.Fatalf("stage 1 must not recommend branch prepare, got %q", got.NextCommand)
	}
	if len(got.Missing) == 0 || got.Missing[0] != "intent_contract" {
		t.Fatalf("stage 1 missing must lead with intent_contract, got %v", got.Missing)
	}
}

func TestClassifyExits(t *testing.T) {
	self := classifyTemplate(withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement)))
	if !strings.HasPrefix(self.Exits.PauseCommand, "issueops execution release") {
		t.Fatalf("self holder must be able to pause, got %q", self.Exits.PauseCommand)
	}
	stale := classifyTemplate(withHolder(baseRecord(issueopsnextcontract.PhaseImplement), "codex", "s-2", false))
	if !strings.HasPrefix(stale.Exits.TakeoverCommand, "issueops execution replace") {
		t.Fatalf("stale holder must be replaceable, got %q", stale.Exits.TakeoverCommand)
	}
	if !strings.HasPrefix(self.Exits.AbandonCommand, "issueops cleanup abandon --id io-test --reason") {
		t.Fatalf("abandon must always be offered, got %q", self.Exits.AbandonCommand)
	}
	none := classifyTemplate(Input{})
	if none.Exits.AbandonCommand != "" {
		t.Fatalf("without a cycle there is nothing to abandon, got %q", none.Exits.AbandonCommand)
	}
}

// 관측하지 못한 홀더는 살아 있는 것으로 본다. 확인하지 않은 세션의 lease를
// 빼앗으라고 권하면 두 세션이 같은 워크트리를 쓴다.
func TestClassifyTreatsUnobservedHolderAsLive(t *testing.T) {
	in := withHolder(baseRecord(issueopsnextcontract.PhaseImplement), "codex", "s-2", true)
	in.HolderLive = nil
	if got := classifyTemplate(in); got.Stage.Key != "blocked.holder_live" {
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
			got := classifyTemplate(in)
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
	if got := classifyTemplate(blank); got.Stage.Key == blockedRoot {
		t.Fatal("a blank root conflict id must not block")
	}
}

func TestClassifyUnknownReportsUnmatchedLocalGates(t *testing.T) {
	got := classifyTemplate(withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseFeedback)), "branch_match", "other"))
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
		got := classifyTemplate(in)
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
		got := classifyTemplate(baseRecord(phase))
		if got.Stage.Key != "issue" || got.Stage.Index != 1 {
			t.Fatalf("%s without issue: got %s/%d want issue/1", phase, got.Stage.Key, got.Stage.Index)
		}
	}
	got := classifyTemplate(baseRecord(issueopsnextcontract.PhasePlan))
	if got.Stage.Key != "unknown" {
		t.Fatalf("plan without issue url must not be treated as stage 1, got %s", got.Stage.Key)
	}
}

func TestClassifyBranchPrepareCommandDependsOnPhase(t *testing.T) {
	problem := classifyTemplate(withIssue(baseRecord(issueopsnextcontract.PhaseProblem)))
	if problem.Stage.Key != "prepare" || problem.NextCommand != "issueops phase --id io-test --to grill $RECORD_ACTOR_FLAGS --json" {
		t.Fatalf("problem with issue: got %s %q", problem.Stage.Key, problem.NextCommand)
	}
	if len(problem.Missing) != 1 || problem.Missing[0] != "intent_contract" {
		t.Fatalf("problem with issue must surface stage-one gates, got %v", problem.Missing)
	}
	grill := classifyTemplate(withIssue(baseRecord(issueopsnextcontract.PhaseGrill)))
	if grill.Stage.Key != "prepare" || strings.Contains(grill.NextCommand, "--to grill") {
		t.Fatalf("grill with issue must not re-advance to grill, got %s %q", grill.Stage.Key, grill.NextCommand)
	}
	if grill.NextCommand != fillBranchPrepareIssue(OwnerCommand("io-test", "branch_prepare"), "https://github.com/acme/repo/issues/12")+" --json" {
		t.Fatalf("grill next command %q want owner command for branch_prepare", grill.NextCommand)
	}
	if len(grill.Missing) != 1 || grill.Missing[0] != "branch_prepare" {
		t.Fatalf("grill missing %v", grill.Missing)
	}
}

// A branch prepared without the provider link check stays in prepare: link-plan
// refuses without branch_link_verified, so letting plan or implement.enter
// through only moves the failure later (an agent run hit exactly that).
func TestClassifyUnverifiedBranchLinkStaysInPrepare(t *testing.T) {
	unverified := func(in Input) Input {
		in = withBranch(in)
		in.Record.BranchPrepare.LinkVerified = false
		return in
	}
	plan := classifyTemplate(unverified(withIssue(baseRecord(issueopsnextcontract.PhasePlan))))
	if plan.Stage.Key != "prepare" || len(plan.Missing) != 1 || plan.Missing[0] != "branch_link_verified" ||
		plan.NextCommand != fillBranchPrepareIssue(OwnerCommand("io-test", "branch_link_verified"), "https://github.com/acme/repo/issues/12")+" --json" || !strings.Contains(plan.NextCommand, " --link-verified ") {
		t.Fatalf("plan with unverified link: %s %v %q", plan.Stage.Key, plan.Missing, plan.NextCommand)
	}
	holder := classifyTemplate(withSelfHolder(unverified(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))))
	if holder.Stage.Key != "prepare" || len(holder.Missing) != 1 || holder.Missing[0] != "branch_link_verified" {
		t.Fatalf("holder at plan with unverified link: %s %v %q", holder.Stage.Key, holder.Missing, holder.NextCommand)
	}
	verified := classifyTemplate(withSelfHolder(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))))
	if verified.Stage.Key != "implement.enter" || len(verified.Missing) != 1 || verified.Missing[0] != "plan_path" {
		t.Fatalf("holder at plan with verified link: %s %v", verified.Stage.Key, verified.Missing)
	}
}

// execution prepare materializes the sealed plan under artifact_dir; the
// plan_path step names that file instead of a <PATH> the agent must hunt for.
func TestClassifyPlanPathNamesTheSealedPlan(t *testing.T) {
	in := withSelfHolder(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan))))
	if got := classifyTemplate(in).NextCommand; got != OwnerCommand("io-test", "plan_path")+" --json" {
		t.Fatalf("without artifact_dir: %q", got)
	}
	in.Record.Execution.Workspace.Root = "/repo.worktrees/9-x"
	in.Record.Execution.Workspace.ArtifactDir = ".issueops/issues/9/artifact"
	if got := classifyTemplate(in).NextCommand; got != "issueops link-plan --id io-test --plan-path '/repo.worktrees/9-x/.issueops/issues/9/artifact/plan.md' $RECORD_ACTOR_FLAGS --json" {
		t.Fatalf("with artifact_dir: %q", got)
	}
}

// The claim stage names the chain's first command directly: a released lease
// starts with replace --preview, a claimable one is claimed with the current
// token. (It used to answer `execution status`, an extra round trip.)
func TestClassifyClaimStageStartsTheChain(t *testing.T) {
	released := classifyTemplate(withReleasedLease(withBranch(withIssue(baseRecord(issueopsnextcontract.PhaseImplement)))))
	if released.Stage.Key != "claim" || released.NextCommand != "issueops execution replace --id 'io-test' --expected-generation 1 --preview --json" {
		t.Fatalf("released: %s %q", released.Stage.Key, released.NextCommand)
	}
	claimable := classifyTemplate(withClaimableLease(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))))
	if claimable.Stage.Key != "claim" || claimable.NextCommand != "issueops execution claim --id 'io-test' --generation 1 --claim-current-token --json" {
		t.Fatalf("claimable: %s %q", claimable.Stage.Key, claimable.NextCommand)
	}
}

func TestClassifyRevokingLeaseOffersTakeover(t *testing.T) {
	revoking := withExecution(baseRecord(issueopsnextcontract.PhaseImplement), issueopscontract.WriteLease{
		Generation: 4, Status: issueopsnextcontract.LeaseStatusRevoking,
	})
	got := classifyTemplate(revoking)
	if got.Stage.Key != "takeover" {
		t.Fatalf("revoking lease stage %s want takeover", got.Stage.Key)
	}
	const replace = "issueops execution replace --id io-test --expected-generation 4 --preview --json"
	if got.NextCommand != replace || got.Exits.TakeoverCommand != replace {
		t.Fatalf("next %q takeover exit %q want %q", got.NextCommand, got.Exits.TakeoverCommand, replace)
	}
	if got.Exits.PauseCommand != "" {
		t.Fatalf("a revoking lease cannot be paused, got %q", got.Exits.PauseCommand)
	}

	self := classifyTemplate(withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement)))
	if self.Exits.TakeoverCommand != "" {
		t.Fatalf("an active self holder must not be offered takeover, got %q", self.Exits.TakeoverCommand)
	}
	live := classifyTemplate(withHolder(baseRecord(issueopsnextcontract.PhaseImplement), "codex", "s-2", true))
	if live.Exits.TakeoverCommand != "" {
		t.Fatalf("a live foreign holder must not be offered takeover, got %q", live.Exits.TakeoverCommand)
	}
}

func TestClassifyTakeoverPrefersWriterlessRecovery(t *testing.T) {
	in := withHolder(baseRecord(issueopsnextcontract.PhaseImplement), "codex", "s-2", false)
	in.WriterlessRecovery = "  issueops execution recover --id io-test  "
	if got := classifyTemplate(in); got.NextCommand != "issueops execution recover --id io-test --json" {
		t.Fatalf("writerless recovery must win, got %q", got.NextCommand)
	}
	in.WriterlessRecovery = "   "
	if got := classifyTemplate(in); got.NextCommand != "issueops execution replace --id io-test --expected-generation 1 --preview --json" {
		t.Fatalf("blank recovery must fall back to replace, got %q", got.NextCommand)
	}
}

func TestClassifyRendersGenerationInTakeoverCommands(t *testing.T) {
	revoking := func(generation uint64) Input {
		return withExecution(baseRecord(issueopsnextcontract.PhaseImplement), issueopscontract.WriteLease{
			Generation: generation, Status: issueopsnextcontract.LeaseStatusRevoking,
		})
	}
	if got := classifyTemplate(revoking(12)); !strings.Contains(got.NextCommand, "--expected-generation 12 ") ||
		!strings.Contains(got.Exits.TakeoverCommand, "--expected-generation 12 ") {
		t.Fatalf("generation 12 must render, got next %q exit %q", got.NextCommand, got.Exits.TakeoverCommand)
	}
	if got := classifyTemplate(revoking(0)); !strings.Contains(got.NextCommand, "--expected-generation <N> ") ||
		!strings.Contains(got.Exits.TakeoverCommand, "--expected-generation <N> ") {
		t.Fatalf("unknown generation must render a placeholder, got next %q exit %q", got.NextCommand, got.Exits.TakeoverCommand)
	}
}

func TestClassifyIssueStageFallsBackToIntentContractWithoutMissingGates(t *testing.T) {
	in := baseRecord(issueopsnextcontract.PhaseGrill)
	in.Completion = func(issueopsnextcontract.Phase) Readiness { return Readiness{Ready: true} }
	got := classifyTemplate(in)
	if got.Stage.Key != "issue" {
		t.Fatalf("stage %s want issue", got.Stage.Key)
	}
	if got.NextCommand != OwnerCommand("io-test", "intent_contract")+" --json" {
		t.Fatalf("next command %q want owner command for intent_contract", got.NextCommand)
	}
}

func TestClassifyWarningsPassThroughLocalAndWorktreeObservations(t *testing.T) {
	const missingRoot = "canonical worktree is missing at /repo.worktrees/12-test; issueops does not rebuild it (the sealed plan artifacts lived there): restore that directory, or end the cycle with exits.abandon_command after releasing the lease (exits.pause_command, --cwd still that path)"
	const missingAfterCompletion = "canonical worktree is missing at /repo.worktrees/12-test"
	withHead := func(finalHead, observed string) Input {
		in := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
		in.Record.Execution.Completion = &issueopscontract.ExecutionCompletion{FinalHead: finalHead}
		in.WorktreeHead, in.WorktreeBranch = observed, "12-test"
		return in
	}
	noCompletion := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
	noCompletion.WorktreeHead, noCompletion.WorktreeBranch = "def", "12-test"
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
		// After completion only cleanup is left, and it handles a missing worktree.
		{"worktree missing after completion", func() Input { in := withHead("abc", ""); in.WorktreePresent = false; return in }(), []string{missingAfterCompletion}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := classifyTemplate(testCase.in).Warnings
			if strings.Join(got, "|") != strings.Join(testCase.want, "|") {
				t.Fatalf("warnings %q want %q", got, testCase.want)
			}
		})
	}

	local := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
	local.Local = &Readiness{Warnings: []string{"base drift"}}
	if got := classifyTemplate(local).Warnings; len(got) != 1 || got[0] != "base drift" {
		t.Fatalf("local warnings must pass through, got %q", got)
	}
	local.WorktreePresent = false
	if got := classifyTemplate(local).Warnings; len(got) != 2 || got[0] != "base drift" || got[1] != missingRoot {
		t.Fatalf("local warnings must precede worktree warnings, got %q", got)
	}
	blankRoot := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
	blankRoot.Record.Execution.Workspace.Root = "  "
	blankRoot.WorktreePresent = false
	if got := classifyTemplate(blankRoot).Warnings; len(got) != 0 {
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
			got := classifyTemplate(step.in)
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
	if got := classifyTemplate(in); got.Stage.Key != "plan.handoff" {
		t.Fatalf("fresh approval must reach handoff, got %q", got.Stage.Key)
	}
	in.ArchitectureReviewStale = true
	got := classifyTemplate(in)
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
	if got := classifyTemplate(child); got.Stage.Key != "plan.review" {
		t.Fatalf("inherited approval must skip the architecture stage, got %q missing=%v", got.Stage.Key, got.Missing)
	}
	child.Record.ArchitectureReview.ReviewerContext = "subagent"
	if got := classifyTemplate(child); got.Stage.Key != "plan.architecture" {
		t.Fatalf("a reviewed child without its own staged architecture stays in plan.architecture, got %q", got.Stage.Key)
	}
}

// After a sync-base merge in pr the head moved and the seals are stale. The
// in-pr fix path runs in order: `phase --to ai-slop-clean` (a refresh that
// stays in pr), the stale reviews, then `phase --to pr` to reseal pr_head,
// which completion compares against. Repeating the refresh would loop.
func TestClassifyPRRefreshSequenceAfterAMovedHead(t *testing.T) {
	pr := func(head string, missing ...string) Input {
		in := withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhasePR)), missing...)
		in.Record.PRHead, in.WorktreeHead = "old", head
		return in
	}
	steps := []struct {
		in         Input
		stage, cmd string
	}{
		{pr("new", "ai_slop_clean_stale", "implementation_review_stale"), "clean", "issueops phase --id io-test --to ai-slop-clean $RECORD_ACTOR_FLAGS --json"},
		{pr("new", "implementation_review_stale"), "verify", OwnerCommand("io-test", "implementation_review_stale") + " --json"},
		{pr("new"), "commit-push", "issueops phase --id io-test --to pr $RECORD_ACTOR_FLAGS --json"},
		{pr("old"), "pr.create", ""},
	}
	for i, step := range steps {
		got := classifyTemplate(step.in)
		if got.Stage.Key != step.stage || (step.cmd != "" && got.NextCommand != step.cmd) {
			t.Fatalf("step %d: stage=%s next=%q", i, got.Stage.Key, got.NextCommand)
		}
	}
}

// Right after `phase --to implement` nothing is implemented yet; the stage
// names what the exit transition still needs instead of an empty missing next
// to the exit command (an agent nearly skipped the implementation).
func TestClassifyImplementNamesWhatTheExitNeeds(t *testing.T) {
	in := withCompletionMissing(withSelfHolder(withBranch(withIssue(baseRecord(issueopsnextcontract.PhaseImplement)))), issueopsnextcontract.PhaseImplement, "implementation_changes")
	got := classifyTemplate(in)
	if got.Stage.Key != "implement" || !slices.Equal(got.Missing, []string{"implementation_changes"}) || !strings.Contains(got.NextCommand, "--to ai-slop-clean") {
		t.Fatalf("stage=%s missing=%v next=%q", got.Stage.Key, got.Missing, got.NextCommand)
	}
}

// The commit-push step is git, which issueops does not run; four agents read
// the readiness re-check as the action. The stage names the git step.
func TestClassifyCommitPushNamesTheGitStep(t *testing.T) {
	in := withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), "upstream", "worktree_clean")
	in.Record.Branch = "12-a"
	got := classifyTemplate(in)
	joined := strings.Join(got.Warnings, "\n")
	if got.Stage.Key != "commit-push" || !strings.Contains(joined, "commit the changes") || !strings.Contains(joined, "git push -u origin 12-a") {
		t.Fatalf("stage=%s warnings=%q", got.Stage.Key, got.Warnings)
	}
}

// create-pr refuses without canonical labels and assignees, and its head/base
// are the sealed branch and base; a pending intent's reconcile needs the
// actor. An agent hit both refusals following the bare templates.
func TestClassifyPRCreateAndReconcileTemplatesNameWhatTheCommandsRequire(t *testing.T) {
	in := withBranch(withSelfHolder(baseRecord(issueopsnextcontract.PhasePR)))
	in.Record.Branch = "12-a"
	got := classifyTemplate(in).NextCommand
	for _, want := range []string{"--head 12-a", "--base main", "--label <LABEL>", "--assignee <USER>", "$ACTOR_FLAGS"} {
		if !strings.Contains(got, want) {
			t.Fatalf("create-pr template %q lacks %q", got, want)
		}
	}
	in.Record.Execution.Pending = &issueopscontract.ExternalIntent{}
	if got := classifyTemplate(in).NextCommand; !strings.Contains(got, "reconcile --id io-test --preview $ACTOR_FLAGS") {
		t.Fatalf("reconcile template %q", got)
	}
}

// A delegated child sat in grill after `branch prepare --link-verified`, and
// next kept handing back the same branch prepare: only the phase was behind.
func TestClassifyGrillWithVerifiedBranchMovesToPlan(t *testing.T) {
	got := classifyTemplate(withBranch(withIssue(baseRecord(issueopsnextcontract.PhaseGrill))))
	if got.Stage.Key != "prepare" || got.NextCommand != "issueops phase --id io-test --to plan $RECORD_ACTOR_FLAGS --json" {
		t.Fatalf("stage=%s next=%q", got.Stage.Key, got.NextCommand)
	}
	unverified := withBranch(withIssue(baseRecord(issueopsnextcontract.PhaseGrill)))
	unverified.Record.BranchPrepare.LinkVerified = false
	if got := classifyTemplate(unverified); !strings.Contains(got.NextCommand, "--link-verified") {
		t.Fatalf("unverified link must still ask for the readback: %q", got.NextCommand)
	}
}

// classifyTemplate classifies and puts the record placeholder back, so rule
// tests pin which command a rule picks, not the caller's rendered flags; the
// rendering has its own test.
func classifyTemplate(in Input) Decision {
	decision := Classify(in)
	if flags := recordActorFlags(in); flags != "" && strings.Contains(decision.NextCommand, flags) {
		decision.NextCommand = strings.ReplaceAll(decision.NextCommand, flags, "$RECORD_ACTOR_FLAGS")
		decision.NextCommandKind = issueopsnextcontract.NextCommandKindTemplate
	}
	return decision
}

// next renders the record vector only where it knows the canonical cwd and the
// caller: before a lease (source checkout) and while the caller holds it
// (worktree). Another holder, a holder with an agent id, or no session keeps
// the template.
func TestClassifyRendersRecordActorFlagsWhenTheVectorIsKnown(t *testing.T) {
	holder := func(in Input, host, session, agent string) Input {
		in.Record.Execution = &issueopscontract.Execution{
			Workspace: issueopscontract.Workspace{Root: "/repo.worktrees/12 test"},
			Lease: issueopscontract.WriteLease{Status: issueopsnextcontract.LeaseStatusActive,
				Holder: &issueopscontract.NativeActor{Host: host, SessionID: session, AgentID: agent}},
		}
		return in
	}
	problem := withIssue(baseRecord(issueopsnextcontract.PhaseProblem))
	if got := Classify(problem); got.NextCommand != "issueops phase --id io-test --to grill --host claude --session-id s-1 --cwd /repo --json" || got.NextCommandKind != "exact" {
		t.Fatalf("pre-lease: %q %s", got.NextCommand, got.NextCommandKind)
	}
	if got := recordActorFlags(holder(problem, "claude", "s-1", "")); got != "--host claude --session-id s-1 --cwd '/repo.worktrees/12 test'" {
		t.Fatalf("holder vector = %q", got)
	}
	for name, in := range map[string]Input{
		"other holder":    holder(problem, "codex", "t-9", ""),
		"agent-id holder": holder(problem, "claude", "s-1", "sub-1"),
		"no session":      func() Input { in := problem; in.ActorSessionID = ""; return in }(),
		"released lease": func() Input {
			in := holder(problem, "claude", "s-1", "")
			in.Record.Execution.Lease.Status = issueopsnextcontract.LeaseStatusReleased
			return in
		}(),
	} {
		if got := recordActorFlags(in); got != "" {
			t.Fatalf("%s: rendered %q, want the template kept", name, got)
		}
	}
}

// A plan revised after its review stays in plan.review before link-plan; the
// implement readiness only compares a linked plan.
func TestClassifyStagedPlanChangedAfterReviewStaysInPlanReview(t *testing.T) {
	in := withDA(withStagedPlan(withArchitecture(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan))))))
	in.DevilsAdvocatePlanStale = true
	got := classifyTemplate(in)
	if got.Stage.Key != "plan.review" || !slices.Equal(got.Missing, []string{"devils_advocate_review_stale"}) {
		t.Fatalf("stage=%q missing=%v", got.Stage.Key, got.Missing)
	}
	if !strings.HasPrefix(got.NextCommand, "issueops devils-advocate review --id io-test") {
		t.Fatalf("next_command=%q", got.NextCommand)
	}
}

// Before a lease the caller is the preparer: the handoff renders its own claim
// vector at the source checkout and becomes exact; without an observable host
// receipt the $ACTOR_FLAGS template stays.
func TestFinishRendersClaimFlagsAtTheHandoff(t *testing.T) {
	in := withDA(withStagedPlan(withArchitecture(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan))))))
	var askedCWD string
	in.ClaimActorFlags = func(cwd string) string {
		askedCWD = cwd
		return "--host claude --session-id s-1 --session-pid 7 --session-started-at t0 --session-executable /bin/claude --cwd " + cwd
	}
	got := Classify(in)
	want := "issueops execution prepare --id io-test --host claude --session-id s-1 --session-pid 7 --session-started-at t0 --session-executable /bin/claude --cwd /repo --json"
	if got.Stage.Key != "plan.handoff" || got.NextCommand != want || got.NextCommandKind != issueopsnextcontract.NextCommandKindExact || askedCWD != "/repo" {
		t.Fatalf("stage=%q kind=%q command=%q cwd=%q", got.Stage.Key, got.NextCommandKind, got.NextCommand, askedCWD)
	}
	in.ClaimActorFlags = func(string) string { return "" }
	if got := Classify(in); !strings.Contains(got.NextCommand, "$ACTOR_FLAGS") || got.NextCommandKind != issueopsnextcontract.NextCommandKindTemplate {
		t.Fatalf("unobservable receipt: kind=%q command=%q", got.NextCommandKind, got.NextCommand)
	}
}

// While the caller holds the lease, holder commands get its claim vector at the
// worktree; the other placeholders keep the command a template.
func TestFinishRendersClaimFlagsForTheHolderAtTheWorktree(t *testing.T) {
	in := withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhasePR)))
	var askedCWD string
	in.ClaimActorFlags = func(cwd string) string {
		askedCWD = cwd
		return "--host claude --session-id s-1 --session-pid 7 --cwd " + cwd
	}
	got := Classify(in)
	root := in.Record.Execution.Workspace.Root
	if got.Stage.Key != "pr.create" || askedCWD != root || strings.Contains(got.NextCommand, "ACTOR_FLAGS") ||
		!strings.HasSuffix(got.NextCommand, "--session-pid 7 --cwd "+root+" --json") || got.NextCommandKind != issueopsnextcontract.NextCommandKindTemplate {
		t.Fatalf("stage=%q kind=%q cwd=%q command=%q", got.Stage.Key, got.NextCommandKind, askedCWD, got.NextCommand)
	}
}

// A resumed session (same host and session id, new process) cannot write
// through the dead process's lease: next routes it to the takeover chain
// instead of holder commands that the CLI refuses.
func TestClassifyResumedSessionWithDeadReceiptTakesOver(t *testing.T) {
	in := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
	dead := false
	in.HolderLive = &dead
	got := classifyTemplate(in)
	if got.Stage.Key != "takeover" || got.Lease.HolderIsSelf ||
		got.NextCommand != "issueops execution replace --id io-test --expected-generation 1 --preview --json" {
		t.Fatalf("stage=%q self=%v command=%q", got.Stage.Key, got.Lease.HolderIsSelf, got.NextCommand)
	}
	alive := true
	in.HolderLive = &alive
	if got := classifyTemplate(in); got.Stage.Key == "takeover" || !got.Lease.HolderIsSelf {
		t.Fatalf("live self holder: stage=%q self=%v", got.Stage.Key, got.Lease.HolderIsSelf)
	}
}

// A worktree switched to another branch is named with the command that puts it
// back; an unobserved branch (no head either) stays silent.
func TestClassifyWarnsWhenTheWorktreeIsOnAnotherBranch(t *testing.T) {
	in := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
	in.WorktreeBranch = "wrong"
	want := "canonical worktree is on branch wrong, not the cycle branch 12-test: switch back with git -C /repo.worktrees/12-test switch 12-test"
	if got := classifyTemplate(in).Warnings; !slices.Contains(got, want) {
		t.Fatalf("warnings %q", got)
	}
	for _, branch := range []string{"", "12-test"} {
		in.WorktreeBranch = branch
		if got := classifyTemplate(in).Warnings; len(got) != 0 {
			t.Fatalf("branch %q: warnings %q", branch, got)
		}
	}
	// Detached HEAD: a head without a branch. A rebase detaches too and is
	// reported by its own warning instead.
	in.WorktreeBranch, in.WorktreeHead = "", "0123456789abcdef"
	if got := classifyTemplate(in).Warnings; len(got) != 1 || !strings.Contains(got[0], "detached HEAD (0123456789), not the cycle branch 12-test") {
		t.Fatalf("detached: warnings %q", got)
	}
	in.GitOperation = "rebase"
	for _, warning := range classifyTemplate(in).Warnings {
		if strings.Contains(warning, "detached HEAD") {
			t.Fatalf("rebase reported as detached: %q", warning)
		}
	}
}

// A caller that does not hold the lease (a resumed session here) has no pause
// command; the warning sends it through the takeover chain to a claimable lease.
func TestClassifyMissingWorktreeWithoutHolderNamesTheTakeoverWay(t *testing.T) {
	in := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
	dead := false
	in.HolderLive = &dead
	in.WorktreePresent = false
	got := classifyTemplate(in)
	if got.Exits.PauseCommand != "" || !slices.ContainsFunc(got.Warnings, func(w string) bool {
		return strings.HasSuffix(w, "after the takeover chain reaches a released lease (stop after --finalize; do not claim)")
	}) {
		t.Fatalf("pause=%q warnings=%q", got.Exits.PauseCommand, got.Warnings)
	}
}

// A parent in implement whose children are not integrated goes to child
// status first; with every child resolved it gets the ai-slop-clean
// transition again.
func TestClassifyImplementParentRoutesToUnintegratedChildren(t *testing.T) {
	in := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
	in.ChildGateMissing = []string{"child_incomplete:io-child"}
	got := classifyTemplate(in)
	if !strings.HasPrefix(got.NextCommand, "issueops child status --parent "+in.Record.ID+" ") || !slices.Contains(got.Missing, "child_incomplete:io-child") {
		t.Fatalf("decision %+v", got)
	}
	in.ChildGateMissing = nil
	if got := classifyTemplate(in); !strings.Contains(got.NextCommand, " --to ai-slop-clean ") {
		t.Fatalf("resolved children: next %q", got.NextCommand)
	}
}

// The claim stage pins the caller's vector with the canonical worktree as
// --cwd, so the claim also runs from the source checkout; without an observable
// receipt it stays the actor-free claim.
func TestClassifyClaimPinsTheCallersVectorAtTheWorktree(t *testing.T) {
	in := withClaimableLease(baseRecord(issueopsnextcontract.PhaseImplement))
	root := in.Record.Execution.Workspace.Root
	in.ClaimActorFlags = func(cwd string) string { return "--host claude --cwd '" + cwd + "'" }
	got := classifyTemplate(in)
	if !strings.HasSuffix(strings.TrimSuffix(got.NextCommand, " --json"), " --claim-current-token --host claude --cwd '"+root+"'") {
		t.Fatalf("claim %q", got.NextCommand)
	}
	in.ClaimActorFlags = func(string) string { return "" }
	if got := classifyTemplate(in); !strings.HasSuffix(strings.TrimSuffix(got.NextCommand, " --json"), " --claim-current-token") {
		t.Fatalf("receipt-less claim %q", got.NextCommand)
	}
}

// After a create-issue whose provider outcome is unknown, next sends the
// cycle to the read-only reconcile preview instead of offering create-issue
// again (a second create could open a duplicate issue).
func TestClassifyIssueStageRoutesAnUnsettledCreateToReconcile(t *testing.T) {
	in := baseRecord(issueopsnextcontract.PhaseGrill)
	in.Record.IssueURL = ""
	in.Record.IssueCreateIntent = &issueopsnextcontract.IssueCreateIntent{Status: issueopsnextcontract.IssueCreateIntentInvokedUnknown}
	got := classifyTemplate(in)
	if got.Stage.Key != issueopsnextcontract.StageIssue || !strings.HasPrefix(got.NextCommand, "issueops remote reconcile-issue --id "+in.Record.ID) || !slices.Contains(got.Missing, "issue_create_unsettled") {
		t.Fatalf("decision %+v", got)
	}
	in.Record.IssueCreateIntent.Status = "completed"
	if got := classifyTemplate(in); strings.Contains(got.NextCommand, "reconcile-issue") {
		t.Fatalf("settled intent still reconciles: %q", got.NextCommand)
	}
}

// Reconcile needs the live holder; with a dead holder the pending intent is
// reached through the takeover chain first, while a live holder (self) still
// gets the reconcile preview.
func TestClassifyPendingWithADeadHolderRoutesToTakeover(t *testing.T) {
	in := withPending(baseRecord(issueopsnextcontract.PhasePR))
	in.Record.Execution.Lease.Holder = &issueopscontract.NativeActor{Host: "codex", SessionID: "s-gone"}
	dead := false
	in.HolderLive = &dead
	got := classifyTemplate(in)
	if got.Stage.Key != issueopsnextcontract.StageTakeover || !strings.Contains(got.NextCommand, "execution replace --id") {
		t.Fatalf("dead holder: %+v", got)
	}
	in = withPending(baseRecord(issueopsnextcontract.PhasePR))
	if got := classifyTemplate(in); got.Stage.Key != issueopsnextcontract.StageBlockedPending || !strings.Contains(got.NextCommand, "execution reconcile") {
		t.Fatalf("live self holder: %+v", got)
	}
	// After the takeover finalized with the intent carried over: claim first.
	in.Record.Execution.Lease.Status = issueopsnextcontract.LeaseStatusClaimable
	in.Record.Execution.Lease.Holder = nil
	if got := classifyTemplate(in); got.Stage.Key != issueopsnextcontract.StageClaim || !strings.Contains(got.NextCommand, "execution claim --id") {
		t.Fatalf("claimable with pending: %+v", got)
	}
}

// An unsettled child create surfaces on every stage; a completed one does not.
func TestClassifyWarnsAboutAnUnsettledChildCreate(t *testing.T) {
	in := Input{Record: issueopsnextcontract.Record{ID: "io-1", Phase: issueopsnextcontract.PhasePlan}}
	in.Record.ChildCreateOperations = []issueopscontract.ChildCreateOperation{{IssueOpsIssueCreateIntent: issueopscontract.IssueOpsIssueCreateIntent{OperationID: "op1", Status: issueopsnextcontract.IssueCreateIntentPending}}}
	if got := Classify(in); !slices.ContainsFunc(got.Warnings, func(w string) bool { return strings.Contains(w, "reconcile-child --id io-1 --operation-id op1") }) {
		t.Fatalf("warnings %q", got.Warnings)
	}
	in.Record.ChildCreateOperations[0].Status = issueopscontract.IssueCreateIntentCompleted
	if got := Classify(in); slices.ContainsFunc(got.Warnings, func(w string) bool { return strings.Contains(w, "reconcile-child") }) {
		t.Fatalf("settled child still warns: %q", got.Warnings)
	}
}

func TestFillBranchPrepareIssue(t *testing.T) {
	template := "issueops branch prepare --id io-1 --provider <github|gitlab> --issue-url <URL> --branch <NAME>"
	for _, tc := range []struct{ url, want string }{
		{"https://github.com/acme/repo/issues/41", "--provider github --issue-url https://github.com/acme/repo/issues/41 --branch <NAME>"},
		{"https://gitlab.example.com/g/r/-/issues/7", "--provider gitlab --issue-url https://gitlab.example.com/g/r/-/issues/7 "},
		{"https://ghe.example.com/acme/repo/issues/41", "--provider <github|gitlab> --issue-url https://ghe.example.com/acme/repo/issues/41 "},
		{"https://github.com/acme/repo/issues/41;rm", "--provider <github|gitlab> --issue-url <URL>"},
		{"", "--issue-url <URL>"},
	} {
		if got := fillBranchPrepareIssue(template, tc.url); !strings.Contains(got, tc.want) {
			t.Fatalf("%q: %q", tc.url, got)
		}
	}
}

// A record that still holds a cleanup attempt resumes at that operation's
// preview, whatever its phase or lease.
func TestClassifyUnfinishedCleanupResumesAtItsPreview(t *testing.T) {
	in := Input{Record: issueopsnextcontract.Record{ID: "io-1", Phase: issueopsnextcontract.PhaseDone}}
	if got := Classify(in); !strings.HasPrefix(got.NextCommand, "issueops remote merge-pr") {
		t.Fatalf("done without attempt: %q", got.NextCommand)
	}
	for op, want := range map[issueopscontract.CleanupOperation]string{
		issueopscontract.CleanupOperationFinish:       "issueops cleanup finish --id io-1 --preview --json",
		issueopscontract.CleanupOperationAbandon:      "issueops cleanup abandon --id io-1 --reason <TEXT> --preview --json",
		issueopscontract.CleanupOperationRemoteBranch: "issueops cleanup remote-branch --id io-1 --preview --json",
	} {
		in.Record.CleanupAttempt = &issueopscontract.IssueOpsCleanupAttempt{Operation: op, Token: "t", StartedAt: "2026-10-25T00:00:00Z"}
		in.Record.Execution = &issueopscontract.Execution{Workspace: issueopscontract.Workspace{Root: "/wt"}}
		got := Classify(in)
		if got.NextCommand != want || got.Stage.Key != issueopsnextcontract.StageCleanupResume {
			t.Fatalf("%s: %s %q", op, got.Stage.Key, got.NextCommand)
		}
		if slices.ContainsFunc(got.Warnings, func(w string) bool { return strings.Contains(w, "canonical worktree is missing") }) {
			t.Fatalf("%s: removed worktree still reads as lost: %q", op, got.Warnings)
		}
	}
}

// A done cycle whose completion was reflected or whose issue was closed is
// past the merge: next continues at the cleanup finish preview.
func TestClassifyDoneAfterCompletionReceiptGoesToFinishPreview(t *testing.T) {
	in := Input{Record: issueopsnextcontract.Record{ID: "io-1", Phase: issueopsnextcontract.PhaseDone}}
	for _, completion := range []issueopscontract.IssueOpsRemoteCompletion{{ReflectedAt: "t"}, {IssueClosedAt: "t"}} {
		in.Record.RemoteCompletion = &completion
		if got := Classify(in); got.NextCommand != "issueops cleanup finish --id io-1 --preview --json" {
			t.Fatalf("%+v: %q", completion, got.NextCommand)
		}
	}
}

// After a stop and regress, the grill step names the re-grill work before
// the phase move back to plan.
func TestClassifyGrillAfterRegressNamesTheReGrill(t *testing.T) {
	in := withBranch(withIssue(baseRecord(issueopsnextcontract.PhaseGrill)))
	got := classifyTemplate(in)
	if slices.ContainsFunc(got.Warnings, func(w string) bool { return strings.Contains(w, "regressed") }) {
		t.Fatalf("no regress yet: %q", got.Warnings)
	}
	in.Record.RegressEvents = []issueopscontract.IssueOpsRegressEvent{{Reason: "stop", FromPhase: issueopsnextcontract.PhasePlan}}
	got = classifyTemplate(in)
	if !strings.Contains(got.NextCommand, "--to plan") || !slices.ContainsFunc(got.Warnings, func(w string) bool { return strings.Contains(w, "contract_change") }) {
		t.Fatalf("after regress: %q %q", got.NextCommand, got.Warnings)
	}
}

// Probe: after sync-base apply paused on conflicts, next named link-plan and
// would have recorded over conflict markers. The holder is sent to finalize.
func TestClassifyRoutesAnUnfinishedMergeToFinalize(t *testing.T) {
	in := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
	in.GitOperation = "merge"
	got := classifyTemplate(in)
	if got.Stage.Key != "implement" || !slices.Equal(got.Missing, []string{"merge_in_progress"}) ||
		!strings.HasPrefix(got.NextCommand, "issueops execution sync-base --id io-test --finalize ") ||
		!slices.ContainsFunc(got.Warnings, func(w string) bool { return strings.Contains(w, "--abort") }) {
		t.Fatalf("stage=%q missing=%v command=%q warnings=%q", got.Stage.Key, got.Missing, got.NextCommand, got.Warnings)
	}
	in.GitOperation = ""
	if got := classifyTemplate(in); slices.Contains(got.Missing, "merge_in_progress") {
		t.Fatalf("no merge: %+v", got)
	}
}

// A rebase, cherry-pick or revert left open is named with its abort command;
// issueops never starts one, so next does not route to it.
func TestClassifyWarnsAboutAnOpenRebase(t *testing.T) {
	in := withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement))
	in.GitOperation = "rebase"
	want := "a git rebase is in progress in the canonical worktree: finish it or run git -C /repo.worktrees/12-test rebase --abort before continuing"
	if got := classifyTemplate(in); !slices.Contains(got.Warnings, want) || slices.Contains(got.Missing, "merge_in_progress") {
		t.Fatalf("warnings=%q missing=%v", got.Warnings, got.Missing)
	}
}

// Probe: merge_conflict recovery on a completed cycle (released lease) paused
// on conflicts and next still offered merge-pr. It routes to the finalize of
// the released completion, with actor flags for the worktree.
func TestClassifyRoutesAReleasedCompletionMergeToFinalize(t *testing.T) {
	in := withReleasedLease(baseRecord(issueopsnextcontract.PhaseDone))
	in.Record.Execution.Completion = &issueopscontract.ExecutionCompletion{Generation: 1, FinalHead: "abc"}
	in.GitOperation = "merge"
	got := classifyTemplate(in)
	if got.Stage.Key != "done" || !slices.Equal(got.Missing, []string{"merge_in_progress"}) ||
		!strings.HasPrefix(got.NextCommand, "issueops execution sync-base --id io-test --completion-generation 1 --finalize ") {
		t.Fatalf("stage=%q missing=%v command=%q", got.Stage.Key, got.Missing, got.NextCommand)
	}
	in.ActorHost, in.ActorSessionID = "claude", "s-9"
	if cwd := actorCWD(in); cwd != in.Record.Execution.Workspace.Root {
		t.Fatalf("actor cwd = %q, want the worktree", cwd)
	}
}
