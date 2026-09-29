package loopresult

import (
	"issueops/cmd/issueops/selfworkflow/progress"
	application "issueops/internal/application/selfverify"
	augmentcontract "issueops/internal/contract/selfaugment"
)

func New(iterations int, baseSeed int64, targetScore float64, root string) augmentcontract.SelfAugmentResult {
	return application.NewLoopResult(iterations, baseSeed, targetScore, root)
}

func EmitStart(reporter *progress.SelfVerifyProgressReporter, loopKind string, iterations int, seed int64) {
	if reporter == nil {
		return
	}
	reporter.Emit(progress.SelfVerifyProgressEvent{
		Event:      "loop_start",
		LoopKind:   loopKind,
		Iterations: iterations,
		Seed:       seed,
	})
}

func EmitEnd(reporter *progress.SelfVerifyProgressReporter, loopKind string, iterations int, seed int64, ok bool, errorText string) {
	if reporter == nil {
		return
	}
	event := progress.SelfVerifyProgressEvent{
		Event:      "loop_end",
		LoopKind:   loopKind,
		Iterations: iterations,
		Seed:       seed,
		OK:         boolPtr(ok),
	}
	if errorText != "" {
		event.Error = errorText
	}
	reporter.Emit(event)
}

func boolPtr(value bool) *bool {
	return &value
}
