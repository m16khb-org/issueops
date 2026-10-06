package issueopsapp

import (
	selfverify "issueops/internal/contract/selfverify"

	"time"

	"issueops/internal/adapter/verification"
)

func runCommandStep(dir, label string, timeout time.Duration, stdin string, name string, args ...string) selfverify.StepResult {
	return verification.Run(dir, label, timeout, stdin, selfVerifyCommandOutputBudgetBytes, name, args...)
}
