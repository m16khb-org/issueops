package mcpcli

import (
	"issueops/internal/adapter/looprun"
)

func init() {
	LoopStart = looprun.Start
	LoopRecordAttempt = looprun.RecordAttempt
	LoopStop = looprun.Stop
	LoopStatus = looprun.Status
}
