package selfworkflow

import (
	verifyapp "issueops/internal/application/selfverify"
	verifycontract "issueops/internal/contract/selfverify"
)

func BuildSelfVerificationContract() SelfVerificationContract {
	return selfVerificationContract()
}

func NewSelfVerifyLoopResult(iterations int, baseSeed int64, targetScore float64) SelfAugmentResult {
	return verifyapp.NewLoopResult(iterations, baseSeed, targetScore, IssueOpsRoot())
}

func EmitSelfVerifyLoopStart(progress *SelfVerifyProgressReporter, loopKind string, iterations int, seed int64) {
	progress.Emit(verifycontract.ProgressEvent{Event: "loop_start", LoopKind: loopKind, Iterations: iterations, Seed: seed})
}

func EmitSelfVerifyLoopEnd(progress *SelfVerifyProgressReporter, loopKind string, iterations int, seed int64, ok bool, errorText string) {
	progress.Emit(verifycontract.ProgressEvent{Event: "loop_end", LoopKind: loopKind, Iterations: iterations, Seed: seed, OK: boolPtr(ok), Error: errorText})
}

func BuildSelfVerificationCoverage(stepLabels []string) ([]SelfVerificationCoverage, []string) {
	return selfVerificationCoverage(stepLabels)
}

func SelfVerificationCoverageDefinitions() []SelfVerificationCoverageDefinition {
	definitions := selfVerificationCoverageDefinitions()
	out := make([]SelfVerificationCoverageDefinition, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, SelfVerificationCoverageDefinition(definition))
	}
	return out
}

func SelfVerificationFailureClusters(result SelfAugmentResult) []SelfVerificationFailureCluster {
	return selfVerificationFailureClusters(result)
}

func SelfVerificationGoalDefinitions() []SelfVerificationGoalDefinition {
	definitions := selfVerificationGoalDefinitions()
	out := make([]SelfVerificationGoalDefinition, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, SelfVerificationGoalDefinition(definition))
	}
	return out
}

func SelfVerifyRerunCommands(failedStep string, baseSeed int64, targetScore float64) []string {
	return selfVerifyRerunCommands(failedStep, baseSeed, targetScore)
}

func SelfVerifyStepRerunCommand(label string) (string, bool) {
	return selfVerifyStepRerunCommand(label)
}
