package issueopsapp

import (
	"issueops/cmd/issueops/selfworkflow"
)

type selfVerifyProgressReporter struct {
	inner *selfworkflow.SelfVerifyProgressReporter
}
