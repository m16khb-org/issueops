package summary

import (
	"issueops/cmd/issueops/selfworkflow/rerun"
	failurecausecontract "issueops/internal/contract/failurecause"
	selfaugmentdomain "issueops/internal/domain/selfaugment"
)

func SummarizeSelfAugment(result SelfAugmentResult) SelfAugmentSummary {
	return SummarizeSelfVerification(result, defaultLoopTargetScoreExclusive)
}

func SummarizeSelfVerification(result SelfAugmentResult, targetScore float64) SelfAugmentSummary {
	runs := make([]selfaugmentdomain.SummaryRun, 0, len(result.Runs))
	for _, run := range result.Runs {
		steps := make([]selfaugmentdomain.SummaryStep, 0, len(run.Steps))
		for _, step := range run.Steps {
			steps = append(steps, selfaugmentdomain.SummaryStep{Label: step.Label, OK: step.OK, DurationMS: step.DurationMS})
		}
		runs = append(runs, selfaugmentdomain.SummaryRun{Iteration: run.Iteration, Seed: run.Seed, Steps: steps})
	}
	summary := selfaugmentdomain.SummarizeSteps(runs, targetScore)
	summary.Contract = SelfVerificationContractValue()
	summary.FailureCauseEvidence = []failurecausecontract.Evidence{}
	summary.GoalScores = MapGoalScores(result, targetScore)
	summary.Coverage, summary.CoverageGaps = SelfVerificationCoverageForLabels(summary.StepLabels)
	if summary.FailedStep != "" {
		summary.RerunCommands = rerun.SelfVerifyRerunCommands(summary.FailedStep, result.BaseSeed, targetScore)
		summary.FailureClass, summary.FailureClassReason, summary.FailureClusters = ClassifySelfVerificationFailure(result, summary)
	}
	evidence := []failurecausecontract.Evidence{}
	for _, run := range result.Runs {
		for _, step := range run.Steps {
			if !step.OK {
				evidence = append(evidence, step.FailureEvidence...)
			}
		}
	}
	cause := Classify(summary.FailedSteps > 0, evidence)
	summary.FailureCause, summary.FailureCauseReason, summary.FailureCauseEvidence = cause.Cause, cause.Reason, cause.Evidence
	selfaugmentdomain.FinalizeSummary(&summary, result.OK)
	return summary
}
