package issueopsapp

import (
	progress "issueops/cmd/issueops/selfworkflow/progress"
)

type selfVerifyProgressReporter struct {
	inner *progress.SelfVerifyProgressReporter
}
