package issueopsnext

// 행렬 차원(Input을 가르는 값 목록)과 사례 생성기(부분 곱, pairwise)다.

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"sort"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
)

// ---------------------------------------------------------------- 차원 정의

type matrixValue struct {
	label string
	apply func(*Input)
	// malformed는 record 검증이 거부하는 모양(예: holder 없는 active lease)이다.
	// 패닉·명령 형식 불변식은 걸지만 stage 기대값 오라클은 걸지 않는다.
	malformed bool
}

type matrixDim struct {
	name   string
	values []matrixValue
}

const (
	dPhase = iota
	dExec
	dGeneration
	dWorkspaceRoot
	dArtifactDir
	dWorktreePresent
	dWorktreeBranch
	dWorktreeHead
	dGitOp
	dCleanup
	dIssueURL
	dBranchPrepare
	dArchReview
	dStagedArch
	dArchStale
	dStagedPlan
	dDevilsAdvocate
	dDevilsAdvocateStale
	dPlanPath
	dCompat
	dProfile
	dLocal
	dArtifact
	dRemoteCompletion
	dIntent
	dChildOps
	dRootConflict
	dChildGate
	dPRHead
	dActor
	dClaimFlags
	dRecovery
	dSourceRoot
	dRepo
	dCount
)

const (
	matrixID       = "io-test"
	matrixBranch   = "12-test"
	matrixRoot     = "/repo.worktrees/12-test"
	matrixRecovery = "issueops execution replace --id io-test --expected-generation 1 --finalize-preview $ACTOR_FLAGS"
	matrixChildOp  = "op-child-1"
)

var matrixPhases = []issueopsnextcontract.Phase{
	issueopsnextcontract.PhaseProblem, issueopsnextcontract.PhaseGrill, issueopsnextcontract.PhasePlan,
	issueopsnextcontract.PhaseCompatibilityReview, issueopsnextcontract.PhaseImplement,
	issueopsnextcontract.PhaseAISlopClean, issueopsnextcontract.PhaseFeedback, issueopsnextcontract.PhasePR,
	issueopsnextcontract.PhaseDone, "bogus-phase",
}

// phaseOrder is the cycle order of the known phases; unknown phases are -1.
func phaseOrder(phase issueopsnextcontract.Phase) int {
	return slices.Index(matrixPhases[:9], phase)
}

func simpleValue(label string, apply func(*Input)) matrixValue {
	return matrixValue{label: label, apply: apply}
}

func buildMatrixDims() []matrixDim {
	dims := make([]matrixDim, dCount)
	dims[dPhase] = matrixDim{"phase", phaseValues()}
	dims[dExec] = matrixDim{"exec", execValues()}
	dims[dGeneration] = matrixDim{"generation", generationValues()}
	dims[dWorkspaceRoot] = matrixDim{"workspace_root", []matrixValue{
		simpleValue("root", nil),
		simpleValue("no-root", func(in *Input) { mutateExecution(in, func(e *issueopscontract.Execution) { e.Workspace.Root = "" }) }),
	}}
	dims[dArtifactDir] = matrixDim{"artifact_dir", []matrixValue{
		simpleValue("artifact-dir", func(in *Input) {
			mutateExecution(in, func(e *issueopscontract.Execution) { e.Workspace.ArtifactDir = ".issueops/sealed" })
		}),
		simpleValue("no-artifact-dir", nil),
	}}
	dims[dWorktreePresent] = matrixDim{"worktree_present", []matrixValue{
		simpleValue("worktree", func(in *Input) { in.WorktreePresent = true }),
		simpleValue("no-worktree", func(in *Input) { in.WorktreePresent = false }),
	}}
	dims[dWorktreeBranch] = matrixDim{"worktree_branch", stringValues("wt-branch", func(in *Input, v string) { in.WorktreeBranch = v },
		matrixBranch, "", "other-branch")}
	dims[dWorktreeHead] = matrixDim{"worktree_head", stringValues("wt-head", func(in *Input, v string) { in.WorktreeHead = v },
		"", "aaaa1111", "bbbb2222")}
	dims[dGitOp] = matrixDim{"git_op", stringValues("git-op", func(in *Input, v string) { in.GitOperation = v },
		"", "merge", "rebase", "cherry-pick", "revert", "bisect")}
	dims[dCleanup] = matrixDim{"cleanup_attempt", cleanupValues()}
	dims[dIssueURL] = matrixDim{"issue_url", stringValues("issue-url", func(in *Input, v string) { in.Record.IssueURL = v },
		"https://github.com/acme/repo/issues/12", "", "https://gitlab.example.com/g/p/-/issues/3", "https://github.com/acme/repo/issues/12?x=1&y=2")}
	dims[dBranchPrepare] = matrixDim{"branch_prepare", branchPrepareValues()}
	dims[dArchReview] = matrixDim{"architecture_review", archReviewValues()}
	dims[dStagedArch] = matrixDim{"staged_architecture", boolValues("staged-arch", func(in *Input, v bool) { in.StagedArchitecture = v }, true)}
	dims[dArchStale] = matrixDim{"architecture_stale", boolValues("arch-stale", func(in *Input, v bool) { in.ArchitectureReviewStale = v }, false)}
	dims[dStagedPlan] = matrixDim{"staged_plan", boolValues("staged-plan", func(in *Input, v bool) { in.StagedPlan = v }, true)}
	dims[dDevilsAdvocate] = matrixDim{"devils_advocate", devilsAdvocateValues()}
	dims[dDevilsAdvocateStale] = matrixDim{"devils_advocate_stale", boolValues("da-stale", func(in *Input, v bool) { in.DevilsAdvocatePlanStale = v }, false)}
	dims[dPlanPath] = matrixDim{"plan_path", stringValues("plan-path", func(in *Input, v string) { in.Record.PlanPath = v },
		"/repo.worktrees/12-test/.issueops/sealed/plan.md", "")}
	dims[dCompat] = matrixDim{"compatibility_review", compatValues()}
	dims[dProfile] = matrixDim{"completion_profile", profileValues()}
	dims[dLocal] = matrixDim{"local_readiness", localValues()}
	dims[dArtifact] = matrixDim{"remote_artifact", []matrixValue{
		simpleValue("no-artifact", nil),
		simpleValue("artifact", func(in *Input) {
			in.Record.RemoteArtifact = &issueopscontract.RemoteArtifactVerification{
				Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/12", VerifiedAt: "2026-09-05T00:03:00Z",
			}
		}),
	}}
	dims[dRemoteCompletion] = matrixDim{"remote_completion", remoteCompletionValues()}
	dims[dIntent] = matrixDim{"issue_create_intent", intentValues()}
	dims[dChildOps] = matrixDim{"child_create_operations", childOpValues()}
	dims[dRootConflict] = matrixDim{"root_conflict", stringValues("root-conflict", func(in *Input, v string) { in.RootConflictID = v }, "", "io-other")}
	dims[dChildGate] = matrixDim{"child_gate_missing", []matrixValue{
		simpleValue("no-child-gate", nil),
		simpleValue("child-gate", func(in *Input) { in.ChildGateMissing = []string{"child_incomplete:c1", "child_unvalidated:c2"} }),
	}}
	dims[dPRHead] = matrixDim{"pr_head", stringValues("pr-head", func(in *Input, v string) { in.Record.PRHead = v }, "", "aaaa1111", "cccc3333")}
	dims[dActor] = matrixDim{"actor", actorValues()}
	dims[dClaimFlags] = matrixDim{"claim_actor_flags", claimFlagValues()}
	dims[dRecovery] = matrixDim{"writerless_recovery", stringValues("recovery", func(in *Input, v string) { in.WriterlessRecovery = v }, "", matrixRecovery)}
	dims[dSourceRoot] = matrixDim{"source_root", stringValues("source-root", func(in *Input, v string) { in.SourceRoot = v }, "/repo", "")}
	dims[dRepo] = matrixDim{"repo", stringValues("repo", func(in *Input, v string) { in.Record.Repo = v }, "/repo", "")}
	return dims
}

func phaseValues() []matrixValue {
	out := make([]matrixValue, 0, len(matrixPhases))
	for _, phase := range matrixPhases {
		out = append(out, simpleValue("phase="+string(phase), func(in *Input) { in.Record.Phase = phase }))
	}
	return out
}

func stringValues(prefix string, set func(*Input, string), values ...string) []matrixValue {
	out := make([]matrixValue, 0, len(values))
	for _, value := range values {
		out = append(out, simpleValue(fmt.Sprintf("%s=%q", prefix, value), func(in *Input) { set(in, value) }))
	}
	return out
}

// boolValues lists the baseline value first.
func boolValues(prefix string, set func(*Input, bool), baseline bool) []matrixValue {
	out := make([]matrixValue, 0, 2)
	for _, value := range []bool{baseline, !baseline} {
		out = append(out, simpleValue(fmt.Sprintf("%s=%v", prefix, value), func(in *Input) { set(in, value) }))
	}
	return out
}

func mutateExecution(in *Input, mutate func(*issueopscontract.Execution)) {
	if in.Record.Execution != nil {
		mutate(in.Record.Execution)
	}
}

func generationValues() []matrixValue {
	out := make([]matrixValue, 0, 3)
	for _, generation := range []uint64{1, 0, 7} {
		out = append(out, simpleValue(fmt.Sprintf("generation=%d", generation), func(in *Input) {
			mutateExecution(in, func(e *issueopscontract.Execution) { e.Lease.Generation = generation })
		}))
	}
	return out
}

type matrixHolder struct {
	label string
	actor *issueopscontract.NativeActor
}

var matrixHolders = []matrixHolder{
	{"self", &issueopscontract.NativeActor{Host: "claude", SessionID: "s-1"}},
	{"other-session", &issueopscontract.NativeActor{Host: "claude", SessionID: "s-2"}},
	{"other-host", &issueopscontract.NativeActor{Host: "codex", SessionID: "s-1"}},
	{"self-with-agent", &issueopscontract.NativeActor{Host: "claude", SessionID: "s-1", AgentID: "agent-1"}},
	{"self-upper-host", &issueopscontract.NativeActor{Host: "CLAUDE", SessionID: "s-1"}},
}

// execValues lists nil first (the baseline), then every valid lease/holder/
// liveness/pending/completion combination, then the malformed shapes.
func execValues() []matrixValue {
	out := []matrixValue{simpleValue("exec=nil", nil)}
	type leaseCase struct {
		status  issueopscontract.LeaseStatus
		holders []matrixHolder
	}
	leases := []leaseCase{
		{issueopsnextcontract.LeaseStatusClaimable, nil},
		{issueopsnextcontract.LeaseStatusReleased, nil},
		{issueopsnextcontract.LeaseStatusActive, matrixHolders},
		{issueopsnextcontract.LeaseStatusRevoking, matrixHolders[:2]},
	}
	for _, lease := range leases {
		for _, holder := range holderCases(lease.holders) {
			for _, live := range liveCases(holder.actor != nil) {
				for _, pending := range []bool{false, true} {
					for _, completion := range []int{0, 1, 2} {
						out = append(out, execValue(lease.status, holder, live, pending, completion, false))
					}
				}
			}
		}
	}
	for _, malformed := range []struct {
		status  issueopscontract.LeaseStatus
		holder  matrixHolder
		pending bool
	}{
		{"", matrixHolder{"none", nil}, false},
		{"bogus", matrixHolders[0], false},
		{issueopsnextcontract.LeaseStatusActive, matrixHolder{"none", nil}, false},
		{issueopsnextcontract.LeaseStatusActive, matrixHolder{"none", nil}, true},
		{issueopsnextcontract.LeaseStatusClaimable, matrixHolders[0], false},
		{issueopsnextcontract.LeaseStatusClaimable, matrixHolders[1], true},
		{issueopsnextcontract.LeaseStatusReleased, matrixHolders[1], false},
		{issueopsnextcontract.LeaseStatusRevoking, matrixHolder{"none", nil}, false},
	} {
		out = append(out, execValue(malformed.status, malformed.holder, nil, malformed.pending, 0, true))
	}
	return out
}

func holderCases(holders []matrixHolder) []matrixHolder {
	if len(holders) == 0 {
		return []matrixHolder{{"no-holder", nil}}
	}
	return holders
}

func liveCases(hasHolder bool) []*bool {
	if !hasHolder {
		return []*bool{nil}
	}
	return []*bool{nil, new(true), new(false)}
}

func liveLabel(live *bool) string {
	switch {
	case live == nil:
		return "unobserved"
	case *live:
		return "live"
	default:
		return "dead"
	}
}

func execValue(status issueopscontract.LeaseStatus, holder matrixHolder, live *bool, pending bool, completion int, malformed bool) matrixValue {
	label := fmt.Sprintf("exec=%s/%s/%s", status, holder.label, liveLabel(live))
	if pending {
		label += "/pending"
	}
	label += fmt.Sprintf("/completion%d", completion)
	return matrixValue{label: label, malformed: malformed, apply: func(in *Input) {
		execution := &issueopscontract.Execution{
			Workspace: issueopscontract.Workspace{SourceRoot: "/repo", Root: matrixRoot, Branch: matrixBranch},
			Lease:     issueopscontract.WriteLease{Generation: 1, Status: status},
		}
		if holder.actor != nil {
			actor := *holder.actor
			execution.Lease.Holder = &actor
		}
		if pending {
			execution.Pending = &issueopscontract.ExternalIntent{OperationID: "op-1", Kind: "create_pr", Marker: "m", StartedAt: "2026-09-05T00:02:00Z"}
		}
		switch completion {
		case 1:
			execution.Completion = &issueopscontract.ExecutionCompletion{Generation: 1, FinalHead: "aaaa1111"}
		case 2:
			execution.Completion = &issueopscontract.ExecutionCompletion{}
		}
		in.Record.Execution = execution
		in.HolderLive = live
	}}
}

func cleanupValues() []matrixValue {
	out := []matrixValue{simpleValue("cleanup=nil", nil)}
	for _, operation := range []issueopscontract.CleanupOperation{
		issueopsnextcontract.CleanupOperationFinish, issueopsnextcontract.CleanupOperationAbandon,
		issueopsnextcontract.CleanupOperationRemoteBranch, "bogus-op",
	} {
		out = append(out, simpleValue("cleanup="+string(operation), func(in *Input) {
			in.Record.CleanupAttempt = &issueopscontract.CleanupAttempt{Operation: operation, StartedAt: "2026-09-05T00:04:00Z"}
		}))
	}
	return out
}

func branchPrepareValues() []matrixValue {
	prepare := func(sha string, verified bool) func(*Input) {
		return func(in *Input) {
			in.Record.BranchPrepare = &issueopscontract.BranchPrepare{
				Provider: "github", IssueURL: in.Record.IssueURL, Branch: matrixBranch, BaseBranch: "main", BaseSHA: sha, LinkVerified: verified,
			}
		}
	}
	return []matrixValue{
		simpleValue("branch-prepare=verified", prepare("a1b2c3d4", true)),
		simpleValue("branch-prepare=nil", nil),
		simpleValue("branch-prepare=no-sha", prepare("", true)),
		simpleValue("branch-prepare=unlinked", prepare("a1b2c3d4", false)),
	}
}

func archReviewValues() []matrixValue {
	review := func(approved bool, context string) func(*Input) {
		return func(in *Input) {
			in.Record.ArchitectureReview = &reviewcontract.ArchitectureReview{
				ProblemSummary: "s", ProposedArchitecture: "a", Approved: approved, ReviewerContext: context,
			}
		}
	}
	return []matrixValue{
		simpleValue("arch-review=approved", review(true, "subagent")),
		simpleValue("arch-review=nil", nil),
		simpleValue("arch-review=unapproved", review(false, "subagent")),
		simpleValue("arch-review=parent-inherited", review(true, issueopsdomain.ParentReviewPattern)),
	}
}

func devilsAdvocateValues() []matrixValue {
	review := func(verdict string, waived bool) func(*Input) {
		return func(in *Input) {
			in.Record.DevilsAdvocateReview = &issueopscontract.DevilsAdvocateReview{
				Verdict: verdict, Waived: waived, ReviewedPlanDigest: "digest", RecordedAt: "2026-09-05T00:01:00Z",
			}
		}
	}
	return []matrixValue{
		simpleValue("da=pass", review("pass", false)),
		simpleValue("da=nil", nil),
		simpleValue("da=revise", review("revise", false)),
		simpleValue("da=stop", review("stop", false)),
		simpleValue("da=stop-waived", review("stop", true)),
		simpleValue("da=STOP-upper", review("STOP", false)),
	}
}

func compatValues() []matrixValue {
	return []matrixValue{
		simpleValue("compat=approved", func(in *Input) {
			in.Record.CompatibilityReview = &reviewcontract.CompatibilityReview{Approved: true}
		}),
		simpleValue("compat=nil", nil),
		simpleValue("compat=unapproved", func(in *Input) {
			in.Record.CompatibilityReview = &reviewcontract.CompatibilityReview{Approved: false}
		}),
	}
}

// profileValues are the injected completion readiness answers per phase.
func profileValues() []matrixValue {
	profile := func(label string, missing map[issueopsnextcontract.Phase][]string) matrixValue {
		return simpleValue("completion="+label, func(in *Input) {
			in.Completion = func(phase issueopsnextcontract.Phase) Readiness {
				keys := missing[phase]
				return Readiness{Ready: len(keys) == 0, Missing: slices.Clone(keys)}
			}
		})
	}
	return []matrixValue{
		profile("ready", nil),
		profile("stage-one-missing", map[issueopsnextcontract.Phase][]string{
			issueopsnextcontract.PhaseProblem: {"intent_contract", "raw_request"},
			issueopsnextcontract.PhaseGrill:   {"branch", "issue_url", "split_decision"},
		}),
		profile("da-missing", map[issueopsnextcontract.Phase][]string{
			issueopsnextcontract.PhaseImplement: {"devils_advocate_review", "implementation_changes"},
		}),
		profile("da-stale", map[issueopsnextcontract.Phase][]string{
			issueopsnextcontract.PhaseImplement: {"devils_advocate_review_stale"},
		}),
		profile("clean-missing", map[issueopsnextcontract.Phase][]string{
			issueopsnextcontract.PhaseAISlopClean: {"ai_slop_clean_at", "cleanup_evidence", "branch"},
		}),
		profile("implement-other-missing", map[issueopsnextcontract.Phase][]string{
			issueopsnextcontract.PhaseImplement: {"implementation_changes", "verification_evidence"},
		}),
	}
}

func localValues() []matrixValue {
	local := func(label string, ready bool, keys []string, warnings ...string) matrixValue {
		return simpleValue("local="+label, func(in *Input) {
			in.Local = &Readiness{Ready: ready, Missing: slices.Clone(keys), Warnings: slices.Clone(warnings)}
		})
	}
	return []matrixValue{
		simpleValue("local=nil", nil),
		local("ready", true, nil),
		local("clean-stale", false, []string{"ai_slop_clean_stale"}),
		local("docs", false, []string{"project_docs_review"}),
		local("upstream", false, []string{"upstream"}),
		local("unmatched", false, []string{"branch_match"}),
		local("implementation-review", false, []string{"implementation_review"}),
		local("gates-incomplete", false, []string{"gates_incomplete:.issueops/gates.md"}),
		local("dirty-and-unpushed", false, []string{"worktree_clean", "upstream"}),
		local("stale-mix", false, []string{"implementation_review_stale", "project_docs_review", "ai_slop_clean_stale"}),
		local("ready-with-warning", true, nil, "base branch moved: 3 commits behind"),
	}
}

func remoteCompletionValues() []matrixValue {
	completion := func(label string, value issueopscontract.RemoteCompletion) matrixValue {
		return simpleValue("remote-completion="+label, func(in *Input) { in.Record.RemoteCompletion = &value })
	}
	return []matrixValue{
		simpleValue("remote-completion=nil", nil),
		completion("merged", issueopscontract.RemoteCompletion{MergedAt: "2026-09-05T01:00:00Z"}),
		completion("reflected", issueopscontract.RemoteCompletion{ReflectedAt: "2026-09-05T01:00:00Z"}),
		completion("issue-closed", issueopscontract.RemoteCompletion{IssueClosedAt: "2026-09-05T01:00:00Z"}),
		completion("empty", issueopscontract.RemoteCompletion{}),
	}
}

var unsettledIntentStatuses = []string{
	issueopsnextcontract.IssueCreateIntentPending, issueopsnextcontract.IssueCreateIntentInvokedUnknown,
	issueopsnextcontract.IssueCreateIntentURLObserved, issueopsnextcontract.IssueCreateIntentVerificationFailed,
	issueopsnextcontract.IssueCreateIntentReceiptFailed,
}

// intentIsUnsettled is the spec: a create whose provider outcome is not recorded
// as settled (the statuses operational health also flags).
func intentIsUnsettled(intent *issueopscontract.IssueCreateIntent) bool {
	return intent != nil && slices.Contains(unsettledIntentStatuses, intent.Status)
}

func intentValues() []matrixValue {
	out := []matrixValue{simpleValue("intent=nil", nil)}
	// pending first among the non-nil values so the small product reaches it.
	for _, status := range append(slices.Clone(unsettledIntentStatuses), "completed", "not_invoked", "") {
		out = append(out, simpleValue("intent="+status, func(in *Input) {
			in.Record.IssueCreateIntent = &issueopscontract.IssueCreateIntent{OperationID: "op-issue", Status: status}
		}))
	}
	return out
}

func childOpValues() []matrixValue {
	op := func(status string) func(*Input) {
		return func(in *Input) {
			in.Record.ChildCreateOperations = []issueopscontract.ChildCreateOperation{{
				IssueCreateIntent: issueopscontract.IssueCreateIntent{OperationID: matrixChildOp, Status: status},
			}}
		}
	}
	return []matrixValue{
		simpleValue("child-ops=none", nil),
		simpleValue("child-ops=unsettled", op(issueopsnextcontract.IssueCreateIntentInvokedUnknown)),
		simpleValue("child-ops=settled", op("completed")),
	}
}

func actorValues() []matrixValue {
	actor := func(host, session string) matrixValue {
		return simpleValue(fmt.Sprintf("actor=%s/%s", host, session), func(in *Input) {
			in.ActorHost, in.ActorSessionID = host, session
		})
	}
	return []matrixValue{actor("claude", "s-1"), actor("claude", ""), actor("", ""), actor("codex", "s-9"), actor("claude", "s-2")}
}

func claimFlagValues() []matrixValue {
	return []matrixValue{
		simpleValue("claim-flags=nil", nil),
		simpleValue("claim-flags=unobservable", func(in *Input) { in.ClaimActorFlags = func(string) string { return "" } }),
		simpleValue("claim-flags=vector", func(in *Input) {
			in.ClaimActorFlags = func(cwd string) string {
				return "--host claude --session-id s-1 --session-pid 7 --session-started-at 2026-09-05T00:00:00Z --session-executable /bin/claude --cwd " + shellWord(cwd)
			}
		}),
	}
}

// ---------------------------------------------------------------- 사례 생성

type matrixCase struct {
	sel   []int
	input Input
	got   Decision
}

type matrix struct {
	dims []matrixDim
}

func (m *matrix) build(sel []int) Input {
	in := Input{
		Record:     issueopsnextcontract.Record{ID: matrixID, Repo: "/repo", Branch: matrixBranch},
		SourceRoot: "/repo", ActorHost: "claude", ActorSessionID: "s-1",
		WorktreePresent: true, WorktreeBranch: matrixBranch,
	}
	for dim, index := range sel {
		if apply := m.dims[dim].values[index].apply; apply != nil {
			apply(&in)
		}
	}
	// Dim 0 of every value list is the baseline; dims whose baseline apply is
	// nil keep the zero Input, so only the completion profile needs a default.
	if in.Completion == nil {
		in.Completion = func(issueopsnextcontract.Phase) Readiness { return Readiness{Ready: true} }
	}
	return in
}

func (m *matrix) label(sel []int) string {
	parts := make([]string, 0, len(sel))
	for dim, index := range sel {
		parts = append(parts, m.dims[dim].values[index].label)
	}
	return strings.Join(parts, " ")
}

func (m *matrix) malformed(sel []int) bool {
	for dim, index := range sel {
		if m.dims[dim].values[index].malformed {
			return true
		}
	}
	return false
}

func (m *matrix) indexOf(dim int, label string) int {
	for index, value := range m.dims[dim].values {
		if value.label == label {
			return index
		}
	}
	panic("unknown matrix value " + label)
}

func (m *matrix) indices(dim int, keep func(matrixValue) bool) []int {
	var out []int
	for index, value := range m.dims[dim].values {
		if keep == nil || keep(value) {
			out = append(out, index)
		}
	}
	return out
}

func allIndices(count int) []int {
	out := make([]int, count)
	for i := range out {
		out[i] = i
	}
	return out
}

// product enumerates the cartesian product of the chosen value indices per
// dim; dims not listed stay at their baseline (index 0).
func (m *matrix) product(choose map[int][]int, visit func(sel []int)) int {
	dims := make([]int, 0, len(choose))
	for dim := range choose {
		dims = append(dims, dim)
	}
	sort.Ints(dims)
	sel := make([]int, len(m.dims))
	count := 0
	var walk func(depth int)
	walk = func(depth int) {
		if depth == len(dims) {
			visit(slices.Clone(sel))
			count++
			return
		}
		for _, index := range choose[dims[depth]] {
			sel[dims[depth]] = index
			walk(depth + 1)
		}
		sel[dims[depth]] = 0
	}
	walk(0)
	return count
}

// pairCover tracks which pairs of dimension values some row already covers.
type pairCover struct {
	sizes     []int
	covered   [][]bool // indexed by i*dims+j for i<j; offset a*sizes[j]+b
	remaining int
	cursor    [3]int // scan position: i, j, offset
}

func newPairCover(sizes []int) *pairCover {
	dimCount := len(sizes)
	p := &pairCover{sizes: sizes, covered: make([][]bool, dimCount*dimCount), cursor: [3]int{0, 1, 0}}
	for i := range dimCount {
		for j := i + 1; j < dimCount; j++ {
			p.covered[i*dimCount+j] = make([]bool, sizes[i]*sizes[j])
			p.remaining += sizes[i] * sizes[j]
		}
	}
	return p
}

func (p *pairCover) slot(i, a, j, b int) (int, int) {
	if i > j {
		i, a, j, b = j, b, i, a
	}
	return i*len(p.sizes) + j, a*p.sizes[j] + b
}

func (p *pairCover) isCovered(i, a, j, b int) bool {
	slot, offset := p.slot(i, a, j, b)
	return p.covered[slot][offset]
}

// nextUncovered scans forward: pairs only ever become covered, so the cursor
// never needs to move back.
func (p *pairCover) nextUncovered() (i, a, j, b int, ok bool) {
	dimCount := len(p.sizes)
	for ; p.cursor[0] < dimCount; p.cursor[0], p.cursor[1] = p.cursor[0]+1, p.cursor[0]+2 {
		for ; p.cursor[1] < dimCount; p.cursor[1], p.cursor[2] = p.cursor[1]+1, 0 {
			pairs := p.covered[p.cursor[0]*dimCount+p.cursor[1]]
			for ; p.cursor[2] < len(pairs); p.cursor[2]++ {
				if !pairs[p.cursor[2]] {
					size := p.sizes[p.cursor[1]]
					return p.cursor[0], p.cursor[2] / size, p.cursor[1], p.cursor[2] % size, true
				}
			}
		}
	}
	return 0, 0, 0, 0, false
}

// cover marks every pair of the finished row.
func (p *pairCover) cover(row []int) {
	for x := range row {
		for y := x + 1; y < len(row); y++ {
			slot, offset := p.slot(x, row[x], y, row[y])
			if !p.covered[slot][offset] {
				p.covered[slot][offset] = true
				p.remaining--
			}
		}
	}
}

// gain counts the uncovered pairs value would add against the assigned dims.
func (p *pairCover) gain(dim, value int, row, assigned []int) int {
	total := 0
	for _, other := range assigned {
		if !p.isCovered(dim, value, other, row[other]) {
			total++
		}
	}
	return total
}

// pairwiseRows returns rows (one value index per dim) that together cover
// every pair of values of every two dims. Deterministic: fixed seed. Each row
// starts from one still-uncovered pair and fills the other dims greedily.
func pairwiseRows(sizes []int) [][]int {
	cover := newPairCover(sizes)
	rng := rand.New(rand.NewPCG(20261104, 7))
	var rows [][]int
	for cover.remaining > 0 {
		i, a, j, b, ok := cover.nextUncovered()
		if !ok {
			break
		}
		row := make([]int, len(sizes))
		for k := range row {
			row[k] = -1
		}
		row[i], row[j] = a, b
		assigned := []int{i, j}
		for _, dim := range rng.Perm(len(sizes)) {
			if row[dim] >= 0 {
				continue
			}
			best, bestGain := 0, -1
			for value := range sizes[dim] {
				if g := cover.gain(dim, value, row, assigned); g > bestGain || (g == bestGain && rng.IntN(2) == 0) {
					best, bestGain = value, g
				}
			}
			row[dim] = best
			assigned = append(assigned, dim)
		}
		cover.cover(row)
		rows = append(rows, row)
	}
	return rows
}
