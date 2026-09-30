package progress

import (
	"io"
	"time"
)

type SelfVerifyProgressReporter struct {
	mode        string
	writer      io.Writer
	started     time.Time
	lastSuccess string
}
