package issueopsapp

import (
	selfverify "issueops/internal/contract/selfverify"

	"time"

	"issueops/internal/adapter/verification"
)

func runCommandStepEnv(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) selfverify.StepResult {
	return verification.RunEnv(dir, label, timeout, stdin, env, selfVerifyCommandOutputBudgetBytes, name, args...)
}
