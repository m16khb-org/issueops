package loopresult

import (
	selfverify "issueops/internal/contract/selfverify"

	"issueops/cmd/issueops/selfworkflow/progress"
)

func EmitStart(reporter *progress.SelfVerifyProgressReporter, loopKind string, iterations int, seed int64) {
	if reporter == nil {
		return
	}
	reporter.Emit(selfverify.ProgressEvent{
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
	event := selfverify.ProgressEvent{
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
