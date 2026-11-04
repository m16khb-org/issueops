package progress

import (
	"io"
	"time"
)

// SelfVerifyProgressReporter writes self-verify progress events to a writer in
// one mode. It is not safe for concurrent use.
type SelfVerifyProgressReporter struct {
	mode        string
	writer      io.Writer
	started     time.Time
	lastSuccess string
}
