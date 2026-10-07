package stateroundtrip

import (
	"fmt"
	"os"
	"time"

	verification "issueops/internal/adapter/verification"
	augmentcontract "issueops/internal/contract/selfaugment"
	verifycontract "issueops/internal/contract/selfverify"
	statecontract "issueops/internal/contract/state"

	verifydomain "issueops/internal/domain/selfverify"
)

const aggregateOutputBudgetBytes = 8 * 1024
const commandOutputBudgetBytes = 32 * 1024

type stateRoundtripCommandRunner func(root, label string, timeout time.Duration, input string, env []string, command ...string) verifycontract.StepResult

type stateRoundtripValidationDeps struct {
	writeRecord   func(string, string, statecontract.RecordEnvelope) (string, error)
	openDatabase  func(string) (StateDatabase, error)
	mkdirTemp     func(string, string) (string, error)
	removeAll     func(string) error
	writeFile     func(string, []byte, os.FileMode) error
	stateRead     func(string, string) (statecontract.StateResult, error)
	writeSnapshot func(string, string, augmentcontract.SelfAugmentStateSnapshot) error
	run           stateRoundtripCommandRunner
}

func (deps stateRoundtripValidationDeps) withDefaults() stateRoundtripValidationDeps {
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.writeFile == nil {
		deps.writeFile = os.WriteFile
	}
	if deps.run == nil {
		deps.run = func(root, label string, timeout time.Duration, input string, env []string, command ...string) verifycontract.StepResult {
			if len(command) == 0 {
				return verifydomain.FailedStep(label, fmt.Errorf("missing command"))
			}
			return runCommandStepEnv(root, label, timeout, input, env, command[0], command[1:]...)
		}
	}
	return deps
}

func runCommandStepEnv(root, label string, timeout time.Duration, input string, env []string, name string, args ...string) verifycontract.StepResult {
	return verification.RunEnv(root, label, timeout, input, env, commandOutputBudgetBytes, name, args...)
}

func assertionStepWithOutput(label string, started time.Time, errs []string, stdoutParts []string, commands []string) verifycontract.StepResult {
	return verifydomain.AssertionStepWithOutput(label, time.Since(started).Milliseconds(), errs, stdoutParts, commands, aggregateOutputBudgetBytes)
}

func combineFailedStep(label string, started time.Time, child verifycontract.StepResult, stdoutParts []string, commands []string) verifycontract.StepResult {
	return verifydomain.CombineFailedStep(label, time.Since(started).Milliseconds(), child, stdoutParts, commands, aggregateOutputBudgetBytes)
}
