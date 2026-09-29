package stateroundtrip

import (
	"fmt"
	"os"
	"time"

	verification "issueops/internal/adapter/verification"
	augmentcontract "issueops/internal/contract/selfaugment"
	verifycontract "issueops/internal/contract/selfverify"
	statecontract "issueops/internal/contract/state"
	augmentdomain "issueops/internal/domain/selfaugment"
	verifydomain "issueops/internal/domain/selfverify"
)

const aggregateOutputBudgetBytes = 8 * 1024
const commandOutputBudgetBytes = 32 * 1024
const selfVerificationSummaryKind = augmentdomain.SelfVerificationSummaryKind

type StepResult = verifycontract.StepResult
type SelfAugmentCompareResult = augmentcontract.SelfAugmentCompareResult
type SelfAugmentHistoryEntry = augmentcontract.SelfAugmentHistoryEntry
type SelfAugmentHistoryResult = augmentcontract.SelfAugmentHistoryResult
type SelfAugmentHistoryRetention = augmentcontract.SelfAugmentHistoryRetention
type SelfAugmentPromoteResult = augmentcontract.SelfAugmentPromoteResult
type SelfAugmentSlowStep = augmentcontract.SelfAugmentSlowStep
type SelfAugmentStateSnapshot = augmentcontract.SelfAugmentStateSnapshot
type SelfAugmentSummary = augmentcontract.SelfAugmentSummary

type stateRoundtripCommandRunner func(root, label string, timeout time.Duration, input string, env []string, command ...string) StepResult

type stateRoundtripValidationDeps struct {
	writeRecord   func(string, string, statecontract.RecordEnvelope) (string, error)
	openDatabase  func(string) (StateDatabase, error)
	mkdirTemp     func(string, string) (string, error)
	removeAll     func(string) error
	writeFile     func(string, []byte, os.FileMode) error
	stateRead     func(string, string) (statecontract.StateResult, error)
	writeSnapshot func(string, string, SelfAugmentStateSnapshot) error
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
		deps.run = func(root, label string, timeout time.Duration, input string, env []string, command ...string) StepResult {
			if len(command) == 0 {
				return failedStep(label, fmt.Errorf("missing command"))
			}
			return runCommandStepEnv(root, label, timeout, input, env, command[0], command[1:]...)
		}
	}
	return deps
}

func runCommandStepEnv(root, label string, timeout time.Duration, input string, env []string, name string, args ...string) StepResult {
	return verification.RunEnv(root, label, timeout, input, env, commandOutputBudgetBytes, name, args...)
}

func failedStep(label string, err error) StepResult {
	return verifydomain.FailedStep(label, err)
}

func assertionStepWithOutput(label string, started time.Time, errs []string, stdoutParts []string, commands []string) StepResult {
	return verifydomain.AssertionStepWithOutput(label, time.Since(started).Milliseconds(), errs, stdoutParts, commands, aggregateOutputBudgetBytes)
}

func combineFailedStep(label string, started time.Time, child StepResult, stdoutParts []string, commands []string) StepResult {
	return verifydomain.CombineFailedStep(label, time.Since(started).Milliseconds(), child, stdoutParts, commands, aggregateOutputBudgetBytes)
}

func tailWithBudget(s string, max int) (string, bool, int) {
	return verifydomain.TailWithBudget(s, max)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
