package selfworkflow

import (
	progress "issueops/cmd/issueops/selfworkflow/progress"
	verifyapp "issueops/internal/application/selfverify"
	augmentcontract "issueops/internal/contract/selfaugment"
	verifycontract "issueops/internal/contract/selfverify"
	verifydomain "issueops/internal/domain/selfverify"
)

func NewSelfVerifyLoopResult(iterations int, baseSeed int64, targetScore float64) augmentcontract.SelfAugmentResult {
	return verifyapp.NewLoopResult(iterations, baseSeed, targetScore, IssueOpsRoot())
}

func EmitSelfVerifyLoopStart(progress *progress.SelfVerifyProgressReporter, loopKind string, iterations int, seed int64) {
	progress.Emit(verifycontract.ProgressEvent{Event: "loop_start", LoopKind: loopKind, Iterations: iterations, Seed: seed})
}

func EmitSelfVerifyLoopEnd(progress *progress.SelfVerifyProgressReporter, loopKind string, iterations int, seed int64, ok bool, errorText string) {
	progress.Emit(verifycontract.ProgressEvent{Event: "loop_end", LoopKind: loopKind, Iterations: iterations, Seed: seed, OK: boolPtr(ok), Error: errorText})
}

func SelfVerificationCoverageDefinitions() []verifycontract.SelfVerificationCoverageDefinition {
	definitions := verifydomain.CoverageDefinitions()
	out := make([]verifycontract.SelfVerificationCoverageDefinition, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, verifycontract.SelfVerificationCoverageDefinition(definition))
	}
	return out
}

func SelfVerificationGoalDefinitions() []verifycontract.SelfVerificationGoalDefinition {
	definitions := verifydomain.GoalDefinitions()
	out := make([]verifycontract.SelfVerificationGoalDefinition, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, verifycontract.SelfVerificationGoalDefinition(definition))
	}
	return out
}
