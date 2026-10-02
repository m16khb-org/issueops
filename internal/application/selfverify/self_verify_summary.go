package selfverify

import (
	failurecausecontract "issueops/internal/contract/failurecause"
	augment "issueops/internal/contract/selfaugment"
	failurecause "issueops/internal/domain/failurecause"
	augmentdomain "issueops/internal/domain/selfaugment"
	domain "issueops/internal/domain/selfverify"
)

func SummarizeSelfVerification(result augment.SelfAugmentResult, targetScore float64) augment.SelfAugmentSummary {
	runs := make([]augmentdomain.SummaryRun, 0, len(result.Runs))
	for _, run := range result.Runs {
		steps := make([]augmentdomain.SummaryStep, 0, len(run.Steps))
		for _, step := range run.Steps {
			steps = append(steps, augmentdomain.SummaryStep{Label: step.Label, OK: step.OK, Reused: step.Reused, DurationMS: step.DurationMS})
		}
		runs = append(runs, augmentdomain.SummaryRun{Iteration: run.Iteration, Seed: run.Seed, Steps: steps})
	}
	summary := augmentdomain.SummarizeSteps(runs, targetScore)
	summary.Contract = domain.ContractValue()
	summary.FailureCauseEvidence = []failurecausecontract.Evidence{}
	summary.GoalScores = MapGoalScores(result, targetScore)
	summary.Coverage, summary.CoverageGaps = domain.CoverageForLabels(summary.StepLabels)
	if summary.FailedStep != "" {
		summary.RerunCommands = domain.SelfVerifyRerunCommands(summary.FailedStep, result.BaseSeed, targetScore)
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
	cause := failurecause.Classify(summary.FailedSteps > 0, evidence)
	summary.FailureCause, summary.FailureCauseReason, summary.FailureCauseEvidence = cause.Cause, cause.Reason, cause.Evidence
	augmentdomain.FinalizeSummary(&summary, result.OK)
	return summary
}
