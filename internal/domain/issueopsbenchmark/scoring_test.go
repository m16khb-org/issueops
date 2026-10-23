package issueopsbenchmark

import (
	"math"
	"reflect"
	"strings"
	"testing"

	contract "issueops/internal/contract/issueopsbenchmark"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func completeArtifact() contract.IssueOpsBenchmarkArtifact {
	const guide = "skills/issueops-create-issue/SKILL.md; skills/issueops-create-pr/SKILL.md"
	return contract.IssueOpsBenchmarkArtifact{
		ProblemSummary: "문제 요약",
		IssueDraft: "## Summary 요약\nBackground 배경\nAcceptance Criteria\nScope\nVerification\n" +
			"selected label: a, rejected label: b, threshold score, manual override",
		Plan:          "run go test",
		TaskBreakdown: "worker owns parser; non-split reason: small",
		TDDPlan:       "write failing test first",
		SubagentPrompts: "You are not alone; do not revert others; own parser. Expected output: report. " +
			"Check pwd, branch, head, worktree; stop on mismatch. Use a verifier.",
		PRDraft:                "Summary 요약\nChanges\nVerified\nReviewer focus\nFixes #1",
		PhaseChoices:           "proceed, revise, jump, pause",
		BranchName:             "feature/x",
		WorktreePath:           "/r/.worktrees/x",
		ImplementationLocation: "/r/.worktrees/x/src",
		WorktreeCleanup:        "clean; cleanup choice offered",
		GuidelineRef:           guide,
		DomainContractEvidence: "invariant; exact mechanism; equivalent behavior; source file: a.go:10",
		APIDocGateEvidence:     "changed endpoint; public error; static check; review; openapi",
		LiveEvidenceMatrix:     "environment; repo config; runtime; remediation order; prod",
		ReviewFeedbackEvidence: "classification valid; verification; thread reply; resolution; kodus resolved=true",
		CompletionHygiene: "final diff; target branch; remote artifact; single commit; cleanup; PR; " +
			"draft issue completion record; evidence; labels; children; unresolved follow-up; pr url",
	}
}

func baseFixture() contract.IssueOpsBenchmarkFixture {
	return contract.IssueOpsBenchmarkFixture{ID: "fx"}
}

func TestScoreArtifactCompleteArtifactPasses(t *testing.T) {
	s := ScoreArtifact(baseFixture(), completeArtifact(), true)
	if !s.Passed || !s.OK || s.MinimumScore != 100 || s.AverageScore != 100 || len(s.DeterministicFailures) != 0 {
		t.Fatalf("expected perfect score: %+v", s)
	}
	if s.FixtureID != "fx" {
		t.Fatalf("fixture id %q", s.FixtureID)
	}
	var na []string
	for _, d := range s.DimensionScores {
		if d.NotApplicable {
			na = append(na, d.Dimension)
		}
	}
	if !reflect.DeepEqual(na, []string{"pioneer_skill_contribution", "skill_routing_fidelity"}) {
		t.Fatalf("n/a dimensions: %v", na)
	}
}

func TestScoreArtifactDimensionFailures(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*contract.IssueOpsBenchmarkArtifact)
		failed string
	}{
		{"no intent", func(a *contract.IssueOpsBenchmarkArtifact) { a.ProblemSummary, a.IssueDraft = " ", "" }, "missing problem summary or issue draft"},
		{"plan without verification", func(a *contract.IssueOpsBenchmarkArtifact) { a.Plan = "ship it" }, "plan missing verification commands"},
		{"phase choices incomplete", func(a *contract.IssueOpsBenchmarkArtifact) { a.PhaseChoices = "proceed, revise, jump" }, "phase choices missing required options"},
		{"branch not feature", func(a *contract.IssueOpsBenchmarkArtifact) { a.BranchName = "main" }, "issue branch or sibling worktree gate missing"},
		{"worktree path lacks .worktrees", func(a *contract.IssueOpsBenchmarkArtifact) {
			a.WorktreePath = "/r/x"
			a.ImplementationLocation = "/r/x"
		}, "issue branch or sibling worktree gate missing"},
		{"impl outside worktree", func(a *contract.IssueOpsBenchmarkArtifact) { a.ImplementationLocation = "/r" }, "implementation location is outside the isolated worktree"},
		{"impl sibling prefix", func(a *contract.IssueOpsBenchmarkArtifact) { a.ImplementationLocation = "/r/.worktrees/x-other" }, "implementation location is outside the isolated worktree"},
		{"tdd without first", func(a *contract.IssueOpsBenchmarkArtifact) { a.TDDPlan = "failing test" }, "TDD plan missing failing-test-first evidence"},
		{"issue lacks hangul", func(a *contract.IssueOpsBenchmarkArtifact) {
			a.IssueDraft = "Summary Background Acceptance Criteria Scope Verification selected label rejected label score manual override"
		}, "issue draft missing required sections, Korean text, guideline reference, or label scoring decision evidence"},
		{"pr without issue link", func(a *contract.IssueOpsBenchmarkArtifact) {
			a.PRDraft = "Summary 요약 Changes Verified Reviewer focus"
		}, "PR/MR draft missing required sections, issue link, Korean text, reviewer notes, or guideline reference"},
		{"cleanup missing choices", func(a *contract.IssueOpsBenchmarkArtifact) { a.WorktreeCleanup = "clean; cleanup" }, "worktree cleanup status or choices missing"},
		{"task breakdown no owns", func(a *contract.IssueOpsBenchmarkArtifact) {
			a.TaskBreakdown = "worker does parser; non-split reason: small"
		}, "task breakdown missing bounded ownership or provider-native hierarchy"},
		{"subagent prompts missing gate", func(a *contract.IssueOpsBenchmarkArtifact) {
			a.SubagentPrompts = "not alone; do not revert; own; report"
		}, "subagent prompts missing coordination, context verification, or bounded review guidance"},
		{"domain evidence missing", func(a *contract.IssueOpsBenchmarkArtifact) { a.DomainContractEvidence = "" }, "domain contract evidence missing invariants, exact mechanisms, equivalent behavior, or source lines"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := completeArtifact()
			c.mutate(&a)
			s := ScoreArtifact(baseFixture(), a, true)
			if s.Passed || s.OK || s.MinimumScore != 0 {
				t.Fatalf("expected failure: %+v", s)
			}
			found := false
			for _, f := range s.DeterministicFailures {
				if f == c.failed {
					found = true
				}
			}
			if !found {
				t.Fatalf("want failure %q in %v", c.failed, s.DeterministicFailures)
			}
		})
	}
}

func TestScoreArtifactConditionalDimensions(t *testing.T) {
	t.Run("routing flag decides routing dimension", func(t *testing.T) {
		fx := baseFixture()
		fx.ExpectedRouting = []contract.SkillRouting{{Skill: "issueops", Phase: "plan"}}
		if s := ScoreArtifact(fx, completeArtifact(), false); s.Passed {
			t.Fatalf("incomplete routing must fail: %+v", s)
		}
		if s := ScoreArtifact(fx, completeArtifact(), true); !s.Passed {
			t.Fatalf("complete routing must pass: %+v", s)
		}
	})
	t.Run("pioneer target without evidence fails", func(t *testing.T) {
		fx := baseFixture()
		fx.PioneerSkillTarget = "database-schema-architecture"
		s := ScoreArtifact(fx, completeArtifact(), true)
		if s.Passed {
			t.Fatalf("missing pioneer evidence must fail: %+v", s)
		}
	})
	t.Run("pioneer target with evidence passes", func(t *testing.T) {
		fx := baseFixture()
		fx.PioneerSkillTarget = "database-schema-architecture"
		a := completeArtifact()
		a.PioneerSkillEvidence = databaseSchemaArchitectureEvidence
		if s := ScoreArtifact(fx, a, true); !s.Passed {
			t.Fatalf("expected pass: %+v", s)
		}
	})
}

func TestScoreArtifactCriticalFailureRules(t *testing.T) {
	cases := []struct {
		name   string
		rule   string
		mutate func(*contract.IssueOpsBenchmarkArtifact)
		hit    bool
	}{
		{"source repo hit", "Works in source repo", func(a *contract.IssueOpsBenchmarkArtifact) { a.ImplementationLocation = "/r" }, true},
		{"source repo miss", "works in source repo", func(a *contract.IssueOpsBenchmarkArtifact) {}, false},
		{"branch prompt hit", "skips branch prompt", func(a *contract.IssueOpsBenchmarkArtifact) { a.BranchName = " " }, true},
		{"worker context hit", "worker starts without context check", func(a *contract.IssueOpsBenchmarkArtifact) { a.SubagentPrompts = "do work" }, true},
		{"unbounded reviewer hit", "unbounded code-reviewer", func(a *contract.IssueOpsBenchmarkArtifact) { a.SubagentPrompts = "launch a code-reviewer" }, true},
		{"dirty worktree removal hit", "removes dirty worktree", func(a *contract.IssueOpsBenchmarkArtifact) { a.WorktreeCleanup = "dirty tree, will remove" }, true},
		{"dirty worktree removal miss", "removes dirty worktree", func(a *contract.IssueOpsBenchmarkArtifact) { a.WorktreeCleanup = "dirty tree kept" }, false},
		{"korean hit", "not written in Korean", func(a *contract.IssueOpsBenchmarkArtifact) { a.PRDraft = "english only" }, true},
		{"guideline hit", "missing issue/pr guideline reference", func(a *contract.IssueOpsBenchmarkArtifact) { a.GuidelineRef = "" }, true},
		{"emoji over threshold", "excessive emoji", func(a *contract.IssueOpsBenchmarkArtifact) { a.IssueDraft += "🚀🚀🚀🚀" }, true},
		{"emoji U+1F300 cyclone counts", "excessive emoji", func(a *contract.IssueOpsBenchmarkArtifact) { a.IssueDraft += strings.Repeat("\U0001F300", 4) }, true},
		{"emoji U+2600 sun counts", "excessive emoji", func(a *contract.IssueOpsBenchmarkArtifact) { a.IssueDraft += strings.Repeat("\u2600", 4) }, true},
		{"emoji U+27BF double curly loop counts", "excessive emoji", func(a *contract.IssueOpsBenchmarkArtifact) { a.IssueDraft += strings.Repeat("\u27BF", 4) }, true},
		{"hangul syllable first 가 is Korean", "not written in Korean", func(a *contract.IssueOpsBenchmarkArtifact) { a.PRDraft = "\uAC00" }, false},
		{"hangul syllable last 힣 is Korean", "not written in Korean", func(a *contract.IssueOpsBenchmarkArtifact) { a.PRDraft = "\uD7A3" }, false},
		{"hangul jamo first ㄱ is Korean", "not written in Korean", func(a *contract.IssueOpsBenchmarkArtifact) { a.PRDraft = "\u3131" }, false},
		{"hangul jamo last ㅎ is Korean", "not written in Korean", func(a *contract.IssueOpsBenchmarkArtifact) { a.PRDraft = "\u314E" }, false},
		{"hangul vowel first ㅏ is Korean", "not written in Korean", func(a *contract.IssueOpsBenchmarkArtifact) { a.PRDraft = "\u314F" }, false},
		{"hangul vowel last ㅣ is Korean", "not written in Korean", func(a *contract.IssueOpsBenchmarkArtifact) { a.PRDraft = "\u3163" }, false},
		{"emoji at threshold", "excessive emoji", func(a *contract.IssueOpsBenchmarkArtifact) { a.IssueDraft += "🚀🚀"; a.PRDraft += "🚀" }, false},
		{"api doc hit", "api doc gate skipped", func(a *contract.IssueOpsBenchmarkArtifact) { a.APIDocGateEvidence = "" }, true},
		{"live evidence hit", "no live evidence", func(a *contract.IssueOpsBenchmarkArtifact) { a.LiveEvidenceMatrix = "" }, true},
		{"review feedback hit", "ignores review feedback", func(a *contract.IssueOpsBenchmarkArtifact) { a.ReviewFeedbackEvidence = "" }, true},
		{"review feedback agent-thread rule routed elsewhere", "ignores review feedback on review-agent threads", func(a *contract.IssueOpsBenchmarkArtifact) {
			a.ReviewFeedbackEvidence = "classification valid; verification; thread reply; resolution"
		}, true},
		{"completion hygiene hit", "skips completion hygiene", func(a *contract.IssueOpsBenchmarkArtifact) { a.CompletionHygiene = "" }, true},
		{"label hit", "no intelligent label decision", func(a *contract.IssueOpsBenchmarkArtifact) { a.IssueDraft = "plain" }, true},
		{"hierarchy hit", "large issue hierarchy ignored", func(a *contract.IssueOpsBenchmarkArtifact) { a.TaskBreakdown = "worker owns parser" }, true},
		{"draft completion hit", "draft issue completion missing", func(a *contract.IssueOpsBenchmarkArtifact) { a.CompletionHygiene = "final diff" }, true},
		{"pioneer hit", "skips pioneer method", func(a *contract.IssueOpsBenchmarkArtifact) {}, true},
		{"unrelated rule never fires", "something else entirely", func(a *contract.IssueOpsBenchmarkArtifact) { *a = contract.IssueOpsBenchmarkArtifact{} }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := baseFixture()
			fx.CriticalFailures = []string{c.rule}
			fx.PioneerSkillTarget = "database-schema-architecture"
			a := completeArtifact()
			c.mutate(&a)
			got := detectIssueOpsCriticalFailures(fx, a, true)
			if c.hit && !reflect.DeepEqual(got, []string{c.rule}) {
				t.Fatalf("want rule reported, got %v", got)
			}
			if !c.hit && len(got) != 0 {
				t.Fatalf("want no failures, got %v", got)
			}
		})
	}
	t.Run("expected routing rule", func(t *testing.T) {
		fx := baseFixture()
		fx.CriticalFailures = []string{"skips expected routing"}
		if got := detectIssueOpsCriticalFailures(fx, completeArtifact(), false); len(got) != 0 {
			t.Fatalf("no expected routing => no failure, got %v", got)
		}
		fx.ExpectedRouting = []contract.SkillRouting{{Skill: "issueops", Phase: "plan"}}
		if got := detectIssueOpsCriticalFailures(fx, completeArtifact(), false); len(got) != 1 {
			t.Fatalf("got %v", got)
		}
		if got := detectIssueOpsCriticalFailures(fx, completeArtifact(), true); len(got) != 0 {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("critical failure blocks pass", func(t *testing.T) {
		fx := baseFixture()
		fx.CriticalFailures = []string{"skips branch prompt"}
		a := completeArtifact()
		a.BranchName = ""
		s := ScoreArtifact(fx, a, true)
		if s.Passed || len(s.CriticalFailures) != 1 {
			t.Fatalf("%+v", s)
		}
	})
}

func TestHierarchyEvidence(t *testing.T) {
	split := "sub-issue with link-child; umbrella issue; [p] parallelizable; depends on A; execution order wave 1"
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"non-split reason alone", "non-split reason: tiny", true},
		{"all parallel split", split, true},
		{"sequential marker without evidence", split + " [s]", false},
		{"sequential marker with evidence", split + " [s] sequential", true},
		{"missing [p]", strings.Replace(split, "[p]", "", 1), false},
		{"missing dependency words", strings.Replace(split, "depends on", "", 1), false},
		{"missing order", strings.Replace(split, "execution order wave 1", "", 1), false},
		{"missing rationale", strings.Replace(split, "umbrella issue", "", 1), false},
		{"collaboration rationale", strings.Replace(split, "umbrella issue", "multiple owners", 1), true},
		{"korean markers", "subissue provider-native; multiple owners; [p] 병렬 가능; 의존; 실행 순서", true},
		{"none", "just do it", false},
	}
	for _, c := range cases {
		if got := issueOpsHierarchyEvidenceComplete(contract.IssueOpsBenchmarkArtifact{TaskBreakdown: c.text}); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}

func TestLabelDecisionEvidence(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"english", "selected label x rejected label y threshold manual override", true},
		{"korean", "선택 라벨 거절 라벨 임계값 수동 판단", true},
		{"mixed pair fails", "selected label x 거절 라벨 y threshold manual override", false},
		{"no threshold", "selected label x rejected label y manual override", false},
		{"no override", "selected label x rejected label y score", false},
	}
	for _, c := range cases {
		if got := issueOpsLabelDecisionEvidenceComplete(contract.IssueOpsBenchmarkArtifact{ProblemSummary: c.text}); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}

func TestReviewPromptIsBounded(t *testing.T) {
	cases := map[string]bool{
		"implement the parser":                             true,
		"run a verifier":                                   true,
		"direct bounded review only":                       true,
		"code-reviewer: do not spawn subagents, 5 minutes": true,
		"code-reviewer: no nested subagents, 시간 예산 10분":    true,
		"code-reviewer: do not spawn subagents":            false,
		"code-reviewer: take 5 minutes":                    false,
		"review everything":                                false,
	}
	for prompt, want := range cases {
		if got := reviewPromptIsBounded(contract.IssueOpsBenchmarkArtifact{SubagentPrompts: prompt}); got != want {
			t.Errorf("%q: got %v", prompt, got)
		}
	}
}

func TestWorkerPromptHasContextGate(t *testing.T) {
	cases := []struct {
		name     string
		prompt   string
		worktree string
		want     bool
	}{
		{"all present", "pwd branch head worktree stop", "", true},
		{"worktree from artifact", "pwd branch head halt", "/w", true},
		{"no worktree anywhere", "pwd branch head stop", "", false},
		{"korean stop", "pwd branch head worktree 중단", "", true},
		{"no stop", "pwd branch head worktree", "", false},
		{"no head", "pwd branch worktree stop", "", false},
	}
	for _, c := range cases {
		a := contract.IssueOpsBenchmarkArtifact{SubagentPrompts: c.prompt, WorktreePath: c.worktree}
		if got := workerPromptHasContextGate(a); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}

func TestTextMatchHelpers(t *testing.T) {
	if !ContainsFold("Hello World", "WORLD") || ContainsFold("abc", "abd") {
		t.Fatal("ContainsFold")
	}
	if !containsAllFold("a b c", "A", "c") || containsAllFold("a b", "a", "z") || !containsAllFold("x") {
		t.Fatal("containsAllFold")
	}
	if !containsAnyFold("a b", "z", "B") || containsAnyFold("a b", "z") || containsAnyFold("a b") {
		t.Fatal("containsAnyFold")
	}
	for s, want := range map[string]bool{"abc": false, "": false, "한": true, "ㅋㅋ": true, "ㅏ": true, "x가y": true} {
		if containsHangul(s) != want {
			t.Errorf("containsHangul(%q)", s)
		}
	}
	concepts := [][]string{{"a", "b"}, {"c"}}
	if !hasAllIssueOpsConcepts("B and C", concepts) || hasAllIssueOpsConcepts("a only", concepts) {
		t.Fatal("hasAllIssueOpsConcepts")
	}
}

func TestSummarizeDimensionScores(t *testing.T) {
	cases := []struct {
		name     string
		in       []contract.IssueOpsDimensionScore
		avg, min float64
	}{
		{"empty", nil, 0, 0},
		{"all n/a", []contract.IssueOpsDimensionScore{{NotApplicable: true, Score: 50}}, 0, 0},
		{"n/a excluded", []contract.IssueOpsDimensionScore{{Score: 100}, {NotApplicable: true, Score: 0}, {Score: 50}}, 75, 50},
		{"first is min", []contract.IssueOpsDimensionScore{{Score: 10}, {Score: 90}}, 50, 10},
	}
	for _, c := range cases {
		avg, min := SummarizeDimensionScores(c.in)
		if !approx(avg, c.avg) || !approx(min, c.min) {
			t.Errorf("%s: %v %v", c.name, avg, min)
		}
	}
}

func TestSummarizeRunScores(t *testing.T) {
	if a, m := SummarizeRunScores(nil); a != 0 || m != 0 {
		t.Fatal("empty")
	}
	a, m := SummarizeRunScores([]contract.IssueOpsBenchmarkScore{
		{AverageScore: 100, MinimumScore: 100}, {AverageScore: 60, MinimumScore: 20}, {AverageScore: 80, MinimumScore: 80},
	})
	if !approx(a, 80) || !approx(m, 20) {
		t.Fatalf("%v %v", a, m)
	}
}

func TestFinalizeRun(t *testing.T) {
	cases := []struct {
		name   string
		scores []contract.IssueOpsBenchmarkScore
		ok     bool
		crit   int
	}{
		{"all pass", []contract.IssueOpsBenchmarkScore{{Passed: true}, {Passed: true}}, true, 0},
		{"one fails", []contract.IssueOpsBenchmarkScore{{Passed: true}, {}}, false, 0},
		{"critical counted", []contract.IssueOpsBenchmarkScore{{Passed: true, CriticalFailures: []string{"a", "b"}}}, false, 2},
		{"empty run is ok", nil, true, 0},
	}
	for _, c := range cases {
		r := FinalizeRun(contract.IssueOpsBenchmarkRunResult{Scores: c.scores, CriticalFailureCount: 99, FixtureCount: 99})
		if r.OK != c.ok || r.CriticalFailureCount != c.crit || r.FixtureCount != len(c.scores) {
			t.Errorf("%s: %+v", c.name, r)
		}
	}
	r := FinalizeRun(contract.IssueOpsBenchmarkRunResult{Scores: []contract.IssueOpsBenchmarkScore{{AverageScore: 90, MinimumScore: 70}, {AverageScore: 50, MinimumScore: 40}}})
	if !approx(r.AverageScore, 70) || !approx(r.MinimumScore, 40) {
		t.Fatalf("%+v", r)
	}
}

func TestScoreRunUsesArtifactsByFixtureID(t *testing.T) {
	fixtures := []contract.IssueOpsBenchmarkFixture{{ID: "a"}, {ID: "b"}}
	run := ScoreRun("run-1", fixtures, map[string]contract.IssueOpsBenchmarkArtifact{"a": completeArtifact()})
	if run.ID != "run-1" || run.FixtureCount != 2 || len(run.Scores) != 2 {
		t.Fatalf("%+v", run)
	}
	if !run.Scores[0].Passed || run.Scores[1].Passed || run.OK {
		t.Fatalf("a must pass, missing b must fail: %+v", run)
	}
	if run.Scores[0].FixtureID != "a" || run.Scores[1].FixtureID != "b" {
		t.Fatal("order not preserved")
	}
}

func TestMergeScoreWithJudge(t *testing.T) {
	det := func() contract.IssueOpsBenchmarkScore {
		return contract.IssueOpsBenchmarkScore{DimensionScores: []contract.IssueOpsDimensionScore{
			{Dimension: "plan_quality", Score: 100, Evidence: "det"},
			{Dimension: "tdd_quality", Score: 100, Evidence: "det2"},
			{Dimension: "phase_control_quality", NotApplicable: true, Evidence: "na"},
		}}
	}
	t.Run("judge lowers score and appends evidence", func(t *testing.T) {
		judge := contract.IssueOpsBenchmarkScore{
			DimensionScores: []contract.IssueOpsDimensionScore{
				{Dimension: "plan_quality", Score: 60, Evidence: "weak"},
				{Dimension: "phase_control_quality", Score: 0, Evidence: "ignored"},
			},
		}
		m := MergeScoreWithJudge(det(), judge)
		if m.DimensionScores[0].Score != 60 || m.DimensionScores[0].Evidence != "det; judge: weak" {
			t.Fatalf("%+v", m.DimensionScores[0])
		}
		if m.DimensionScores[1].Score != 100 || m.DimensionScores[1].Evidence != "det2" {
			t.Fatalf("unjudged dimension must be untouched: %+v", m.DimensionScores[1])
		}
		if m.DimensionScores[2].Evidence != "na" || m.DimensionScores[2].Score != 0 {
			t.Fatalf("n/a must be untouched: %+v", m.DimensionScores[2])
		}
		if m.Passed || m.OK || !approx(m.MinimumScore, 60) || !approx(m.AverageScore, 80) {
			t.Fatalf("%+v", m)
		}
	})
	t.Run("judge never raises score", func(t *testing.T) {
		d := det()
		d.DimensionScores[0].Score = 40
		m := MergeScoreWithJudge(d, contract.IssueOpsBenchmarkScore{DimensionScores: []contract.IssueOpsDimensionScore{{Dimension: "plan_quality", Score: 100, Evidence: "ok"}}})
		if m.DimensionScores[0].Score != 40 {
			t.Fatalf("%+v", m.DimensionScores[0])
		}
	})
	t.Run("agreeing judge passes", func(t *testing.T) {
		m := MergeScoreWithJudge(det(), contract.IssueOpsBenchmarkScore{DimensionScores: []contract.IssueOpsDimensionScore{{Dimension: "plan_quality", Score: 100, Evidence: "ok"}}})
		if !m.Passed || !m.OK {
			t.Fatalf("%+v", m)
		}
	})
	t.Run("empty judge records failure", func(t *testing.T) {
		m := MergeScoreWithJudge(det(), contract.IssueOpsBenchmarkScore{})
		if m.Passed || !reflect.DeepEqual(m.JudgeFailures, []string{"judge returned no dimension scores"}) {
			t.Fatalf("%+v", m)
		}
	})
	t.Run("judge failures and critical failures propagate", func(t *testing.T) {
		m := MergeScoreWithJudge(det(), contract.IssueOpsBenchmarkScore{
			DimensionScores:  []contract.IssueOpsDimensionScore{{Dimension: "plan_quality", Score: 100}},
			JudgeFailures:    []string{"jf"},
			CriticalFailures: []string{"cf"},
		})
		if m.Passed || len(m.JudgeFailures) != 1 || len(m.CriticalFailures) != 1 {
			t.Fatalf("%+v", m)
		}
	})
	t.Run("deterministic failure blocks pass", func(t *testing.T) {
		d := det()
		d.DeterministicFailures = []string{"x"}
		m := MergeScoreWithJudge(d, contract.IssueOpsBenchmarkScore{DimensionScores: []contract.IssueOpsDimensionScore{{Dimension: "plan_quality", Score: 100}}})
		if m.Passed {
			t.Fatal("must not pass")
		}
	})
}

func TestValidateJudgeScore(t *testing.T) {
	if err := ValidateJudgeScore(contract.IssueOpsBenchmarkScore{}); err == nil || !strings.Contains(err.Error(), "dimension_scores") {
		t.Fatalf("%v", err)
	}
	if err := ValidateJudgeScore(contract.IssueOpsBenchmarkScore{DimensionScores: []contract.IssueOpsDimensionScore{{}}}); err != nil {
		t.Fatal(err)
	}
}

func TestJudgeMetadataErrors(t *testing.T) {
	cases := []struct {
		name  string
		judge contract.IssueOpsJudgeMap
		run   string
		want  string
	}{
		{"missing source", contract.IssueOpsJudgeMap{Provenance: "p"}, "r", "missing source_run_id"},
		{"blank source", contract.IssueOpsJudgeMap{SourceRunID: "  ", Provenance: "p"}, "r", "missing source_run_id"},
		{"missing provenance", contract.IssueOpsJudgeMap{SourceRunID: "s", Provenance: " "}, "r", "missing provenance"},
		{"self attributed with whitespace", contract.IssueOpsJudgeMap{SourceRunID: " r ", Provenance: "p"}, "r ", "is the scored run itself"},
	}
	for _, c := range cases {
		_, err := ValidateJudgeMetadata(c.judge, c.run)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}
