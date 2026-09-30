package contractauditworker

import (
	"os"
	"time"

	verification "issueops/internal/adapter/verification"
	verifycontract "issueops/internal/contract/selfverify"
)

const commandOutputBudgetBytes = 32 * 1024

type ValidationDeps struct {
	MkdirTemp         func(string, string) (string, error)
	RemoveAll         func(string) error
	ReadFile          func(string) ([]byte, error)
	RunCommandStep    func(string, string, time.Duration, string, string, ...string) verifycontract.StepResult
	RunCommandStepEnv func(string, string, time.Duration, string, []string, string, ...string) verifycontract.StepResult
}

func (deps ValidationDeps) withDefaults() ValidationDeps {
	if deps.MkdirTemp == nil {
		deps.MkdirTemp = os.MkdirTemp
	}
	if deps.RemoveAll == nil {
		deps.RemoveAll = os.RemoveAll
	}
	if deps.ReadFile == nil {
		deps.ReadFile = os.ReadFile
	}
	if deps.RunCommandStep == nil {
		deps.RunCommandStep = func(dir, label string, timeout time.Duration, stdin string, name string, args ...string) verifycontract.StepResult {
			return verification.Run(dir, label, timeout, stdin, commandOutputBudgetBytes, name, args...)
		}
	}
	if deps.RunCommandStepEnv == nil {
		deps.RunCommandStepEnv = func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) verifycontract.StepResult {
			return verification.RunEnv(dir, label, timeout, stdin, env, commandOutputBudgetBytes, name, args...)
		}
	}
	return deps
}
