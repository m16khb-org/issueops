package selfaugment

import contract "issueops/internal/contract/selfaugment"

type PlanFacts struct {
	GeniusText          string
	Signals             contract.SelfAugmentRepoSignals
	DocsOK              bool
	ImplementationDelta bool
	VerificationPassed  bool
	LessonsCaptured     bool
}

func NewPlan(req contract.SelfAugmentPlanRequest, facts PlanFacts, candidates []contract.SelfAugmentCandidate, lessonCounts map[string]int) contract.SelfAugmentPlanResult {
	goals := []contract.SelfAugmentGoal{
		{
			Name: "curriculum_selection", KoreanName: "개선 목표 선별", TargetScore: req.TargetScore,
			Score:       ScoreBool(facts.Signals.HasGeniusThink && facts.DocsOK),
			Description: "GENIUS_THINK.md와 repo evidence로 10개 이상 후보를 만들고 가치·위험·실현 가능성을 수치화한다.",
			Evidence:    []string{"GENIUS_THINK.md", "docs index", "skill inventory"},
		},
		{
			Name: "implementation_delta", KoreanName: "개선 구현", TargetScore: req.TargetScore,
			Score:       ScoreBool(facts.ImplementationDelta),
			Description: "선택 후보를 실제 코드/문서/스킬 diff로 구현한다. 단순 보고서만으로는 통과하지 않는다.",
			Evidence:    ImplementationEvidence(facts.ImplementationDelta),
		},
		{
			Name: "verification_qa", KoreanName: "검증·QA", TargetScore: req.TargetScore,
			Score:       ScoreBool(facts.VerificationPassed),
			Description: "Targeted tests, QA smoke checks, and the self-verification loop must pass.",
			Evidence:    VerificationEvidence(facts.VerificationPassed),
		},
		{
			Name: "learning_capture", KoreanName: "학습 기록", TargetScore: req.TargetScore,
			Score:       ScoreBool(facts.LessonsCaptured),
			Description: "실패/성공 원인과 다음 개선점을 state/docs 중 적절한 위치에 남긴다.",
			Evidence:    LearningEvidence(facts.LessonsCaptured),
		},
	}
	for i := range goals {
		goals[i].Passed = GoalPassed(goals[i].Score, goals[i].TargetScore)
	}
	warnings := ApplyLessonPenalties(candidates, lessonCounts)
	PrioritizeCandidates(candidates)
	selected := SelectedCandidate(candidates)
	return contract.SelfAugmentPlanResult{
		OK:                  true,
		LoopKind:            "self_augmentation",
		KoreanName:          contract.SelfAugmentationKoreanName,
		Cycles:              req.Cycles,
		TargetScore:         req.TargetScore,
		TerminationEligible: AllGoalsPassed(goals),
		UsesGeniusThink:     facts.Signals.HasGeniusThink,
		SelectedFormulas:    SelectGeniusFormulas(facts.GeniusText),
		ResearchInfluences:  ResearchInfluences(),
		Goals:               goals,
		Candidates:          candidates,
		SelectedCandidate:   selected,
		ExecutionProtocol: []string{
			"baseline: run the self-verification loop and collect goal scores",
			"curriculum: generate at least 10 improvement candidates using GENIUS_THINK.md formulas and repo evidence",
			"selection: choose the highest score safe candidate, not the easiest cosmetic change",
			"implementation: edit code/docs/skills with small reversible diffs",
			"feedback: convert failures into Reflexion-style verbal lessons and retry within the cycle budget",
			"termination: stop only when every self-augmentation goal and self-verification goal scores above target_score",
		},
		VerificationGate: []string{
			"targeted tests for changed behavior",
			"QA gate including docs, skills, native integration, and MCP/state smoke",
			"go test ./... -count=1",
			"go test -race ./... -count=1 when touched Go behavior is concurrency or policy sensitive",
			"./bin/issueops self-verify --target-score=95 --json",
		},
		Warnings:    warnings,
		RepoSignals: facts.Signals,
	}
}
