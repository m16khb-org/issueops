package issueopscycle

import (
	"slices"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	port "issueops/internal/port/issueopscycle"
)

type baseAdvancedGit struct {
	readinessGitFixture
	refs []string
}

func (g *baseAdvancedGit) BaseAdvanced(_, ref string) bool {
	g.refs = append(g.refs, ref)
	return true
}

func feedbackRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{Repo: "/repo", Branch: "feature", Phase: model.IssueOpsPhaseFeedback, Execution: &model.Execution{}}
}

func TestAISlopCleanReadinessReportsMissingImplementationChangesOnlyWithoutEvidence(t *testing.T) {
	for _, tc := range []struct {
		name        string
		hasEvidence bool
	}{{"no evidence", false}, {"with evidence", true}} {
		t.Run(tc.name, func(t *testing.T) {
			locked, events := false, []string{}
			ready, _ := readinessForEffects(t, &locked, &events)
			ready.HasEvidence = func(model.IssueOpsRecord) bool { return tc.hasEvidence }
			got := ready.AISlopClean(feedbackRecord())
			if slices.Contains(got.Missing, "implementation_changes") == tc.hasEvidence {
				t.Fatalf("hasEvidence=%v missing=%v", tc.hasEvidence, got.Missing)
			}
			if slices.Contains(got.Missing, "") {
				t.Fatalf("empty key leaked into missing: %q", got.Missing)
			}
			if got.Ready {
				t.Fatalf("an unprepared record must not be ready: %+v", got)
			}
		})
	}
}

func TestPRReadinessIsNotReadyWhileLocalGatesAreMissing(t *testing.T) {
	locked, events := false, []string{}
	ready, _ := readinessForEffects(t, &locked, &events)
	got := ready.PR(model.IssueOpsRecord{})
	if got.Ready || len(got.Missing) == 0 || !slices.Contains(got.Missing, "issue_url") {
		t.Fatalf("empty record must report missing gates and not be ready: %+v", got)
	}
}

func TestStrictPRWithFetchAppliesChildGateWhenEitherMissingOrWarningsExist(t *testing.T) {
	for _, tc := range []struct {
		name         string
		missing      []string
		warnings     []string
		wantMissing  bool
		wantWarnings bool
	}{
		{"missing only", []string{"child_pr_open"}, nil, true, false},
		{"warnings only", nil, []string{"child #4 still open"}, false, true},
		{"neither", nil, nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			locked, events := false, []string{}
			ready, _ := readinessForEffects(t, &locked, &events)
			ready.ChildMissing = func(string, model.IssueOpsRecord) ([]string, []string) { return tc.missing, tc.warnings }
			got := ready.StrictPRWithFetch("state", feedbackRecord(), ready.Git.Fetch)
			if slices.Contains(got.Missing, "child_pr_open") != tc.wantMissing || slices.Contains(got.Warnings, "child #4 still open") != tc.wantWarnings {
				t.Fatalf("missing=%v warnings=%v", got.Missing, got.Warnings)
			}
		})
	}
}

func TestObservedPRBaseAdvancedWarnsOnlyWhenPreparedBaseIsKnown(t *testing.T) {
	locked, events := false, []string{}
	ready, _ := readinessForEffects(t, &locked, &events)
	git := &baseAdvancedGit{readinessGitFixture: readinessGitFixture{root: "/repo", locked: &locked, events: &events, t: t}}
	ready.Git = git

	withBase := feedbackRecord()
	withBase.BranchPrepare = &model.IssueOpsBranchPrepare{BaseBranch: "refs/heads/main"}
	got, _ := ready.ObserveLocalPR(withBase)
	if !slices.Equal(git.refs, []string{"origin/main"}) || !slices.ContainsFunc(got.Warnings, func(w string) bool { return strings.HasPrefix(w, "base_advanced: origin/main ") }) {
		t.Fatalf("refs=%v warnings=%v", git.refs, got.Warnings)
	}

	git.refs = nil
	got, _ = ready.ObserveLocalPR(feedbackRecord())
	if len(git.refs) != 0 || slices.ContainsFunc(got.Warnings, func(w string) bool { return strings.HasPrefix(w, "base_advanced:") }) {
		t.Fatalf("no prepared base: refs=%v warnings=%v", git.refs, got.Warnings)
	}
}

func TestObservedPRProbesPlanAndWorktreeOnlyWhenRecordNamesThem(t *testing.T) {
	var planCalls, worktreeCalls []string
	newReady := func() Readiness {
		locked, events := false, []string{}
		ready, _ := readinessForEffects(t, &locked, &events)
		ready.Paths = port.ReadinessObservations{
			WorktreePathValid:        func(path string) bool { worktreeCalls = append(worktreeCalls, path); return true },
			PlanPathExists:           func(_, path string) bool { planCalls = append(planCalls, path); return true },
			PlanInLinkedWorktree:     func(model.IssueOpsRecord) bool { return true },
			WorkspaceMatches:         func(string, string) bool { return true },
			LinkedPlanDigest:         func(model.IssueOpsRecord) (string, error) { return "", nil },
			SealedArchitectureDigest: func(model.IssueOpsRecord) (string, error) { return "architecture-digest", nil },
		}
		return ready
	}

	t.Run("named paths are verified", func(t *testing.T) {
		record := feedbackRecord()
		record.PlanPath, record.WorktreePath = "plans/p.md", "/repo"
		got, _ := newReady().ObserveLocalPR(record)
		for _, key := range []string{"plan_exists", "worktree_exists", "worktree_path"} {
			if slices.Contains(got.Missing, key) {
				t.Fatalf("existing plan and worktree must not be reported as %s: %v", key, got.Missing)
			}
		}
		if !slices.Contains(worktreeCalls, "/repo") || !slices.Contains(planCalls, "plans/p.md") {
			t.Fatalf("plan=%v worktree=%v", planCalls, worktreeCalls)
		}
	})
	t.Run("blank paths are never probed", func(t *testing.T) {
		planCalls, worktreeCalls = nil, nil
		record := feedbackRecord()
		record.PlanPath, record.WorktreePath = "  ", "\t"
		got, _ := newReady().ObserveLocalPR(record)
		if len(planCalls) != 0 || len(worktreeCalls) != 0 {
			t.Fatalf("blank paths probed: plan=%q worktree=%q", planCalls, worktreeCalls)
		}
		if !slices.Contains(got.Missing, "worktree_path") {
			t.Fatalf("blank worktree path must be reported: %v", got.Missing)
		}
	})
}

func implementReadyRecord() model.IssueOpsRecord {
	holder := model.NativeActor{Host: "codex", SessionID: "session", SessionProcess: &model.NativeProcessReceipt{PID: 42, StartedAt: "start", Executable: "/bin/codex"}}
	return model.IssueOpsRecord{
		ID: "io-1", Repo: "/repo", Branch: "feature", Phase: model.IssueOpsPhaseCompatibilityReview,
		IssueURL: "https://github.com/acme/repo/issues/1", PlanPath: "plans/p.md", WorktreePath: "/repo.worktrees/feature",
		BranchPrepare: &model.IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/acme/repo/issues/1", Branch: "feature", BaseBranch: "main", LinkVerified: true},
		Intent:        &model.IssueOpsIntentContract{RawRequest: "r", InterpretedIntent: "i", SuccessCriteria: []string{"c"}},
		ArchitectureReview: &reviewcontract.ArchitectureReview{
			ProblemSummary: "p", ProposedArchitecture: "d", RefactorPlan: "r", Alternatives: []string{"a"}, Risks: []string{"r"},
			Verification: []string{"architecture review checked alternatives and risks"}, Approved: true,
			ReviewerContext: "subagent", ReviewedArchitectureDigest: "architecture-digest",
		},
		CompatibilityReview:  &reviewcontract.CompatibilityReview{BackwardCompatibility: []string{"b"}, SideEffects: []string{"s"}, RollbackPlan: "r", Verification: []string{"v"}, Approved: true},
		DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{Verdict: "proceed", RecordedAt: "2026-10-12T00:00:00Z", ReviewedPlanDigest: "digest"},
		Execution: &model.Execution{
			Workspace: model.Workspace{SourceRoot: "/repo", Root: "/repo.worktrees/feature", Branch: "feature", BaseHead: "base", LinkedAt: "linked"},
			Lease:     model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &holder, ClaimedAt: "claimed"},
		},
	}
}

func TestAdvanceReportWritesTrackedMaterialsWhenEnteringImplementIfHookWired(t *testing.T) {
	for _, tc := range []struct {
		name      string
		hook      bool
		wantCalls int
	}{{"hook wired", true, 1}, {"hook absent", false, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			locked, events := false, []string{}
			ready, _ := readinessForEffects(t, &locked, &events)
			ready.Paths.LinkedPlanDigest = func(model.IssueOpsRecord) (string, error) { return "digest", nil }
			ready.Paths.SealedArchitectureDigest = func(model.IssueOpsRecord) (string, error) { return "architecture-digest", nil }
			p := &phaseFlow{record: implementReadyRecord()}
			service := flowService(p, ready)
			calls := 0
			if tc.hook {
				service.WriteMaterials = func(record model.IssueOpsRecord) model.IssueOpsTrackedMaterials {
					calls++
					if record.Phase != model.IssueOpsPhaseCompatibilityReview {
						t.Fatalf("materials must be written from the pre-transition record, got phase %s", record.Phase)
					}
					return model.IssueOpsTrackedMaterials{}
				}
			}
			got, _, err := service.AdvanceReport("state", "io-1", "implement")
			if err != nil {
				t.Fatal(err)
			}
			if calls != tc.wantCalls || got.Phase != model.IssueOpsPhaseImplement || len(p.writes) != 1 {
				t.Fatalf("calls=%d phase=%s writes=%d", calls, got.Phase, len(p.writes))
			}
		})
	}
}

type rewrittenBaseGit struct{ baseAdvancedGit }

func (g *rewrittenBaseGit) Contains(string, string, string) bool { return false }

// Probed: origin/main was force-pushed past the sealed base; readiness
// pointed at sync-base as for an advanced base, while the PR would carry
// the commits the base dropped.
func TestStrictPRReadinessNamesARewrittenBase(t *testing.T) {
	locked, events := false, []string{}
	ready, _ := readinessForEffects(t, &locked, &events)
	git := &rewrittenBaseGit{baseAdvancedGit: baseAdvancedGit{readinessGitFixture: *ready.Git.(*readinessGitFixture)}}
	ready.Git = git
	record := feedbackRecord()
	record.BranchPrepare = &model.IssueOpsBranchPrepare{BaseBranch: "main", BaseSHA: "sealed"}
	got, _ := ready.observePR(record, nil)
	joined := strings.Join(got.Warnings, "\n")
	if !strings.Contains(joined, "base_rewritten: origin/main") || strings.Contains(joined, "base_advanced") {
		t.Fatalf("warnings = %q", got.Warnings)
	}
}
