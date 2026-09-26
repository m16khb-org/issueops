package progress

import (
	"io"
	"time"

	selfverifycontract "issueops/internal/contract/selfverify"
)

type SelfVerifyProgressEvent = selfverifycontract.ProgressEvent

type SelfVerifyProgressReporter struct {
	mode        string
	writer      io.Writer
	started     time.Time
	lastSuccess string
}
