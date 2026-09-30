package commandpolicy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	verification "issueops/internal/adapter/verification"
	verifycontract "issueops/internal/contract/selfverify"
	verifydomain "issueops/internal/domain/selfverify"
)

const aggregateOutputBudgetBytes = 8 * 1024
const commandOutputBudgetBytes = 32 * 1024

type commandPolicyCommandRunner func(dir, label string, timeout time.Duration, stdin string, name string, args ...string) verifycontract.StepResult

type commandPolicyValidationDeps struct {
	makeTempDir func(kind string) (string, error)
	removeAll   func(path string) error
	exists      func(path string) bool
	run         commandPolicyCommandRunner
}

func (deps commandPolicyValidationDeps) withDefaults() commandPolicyValidationDeps {
	if deps.makeTempDir == nil {
		deps.makeTempDir = func(kind string) (string, error) {
			switch kind {
			case "workspace":
				return os.MkdirTemp("", "issueops-policy-*")
			case "outside":
				return os.MkdirTemp("", "issueops-policy-outside-*")
			default:
				return "", fmt.Errorf("unknown command policy temp kind: %s", kind)
			}
		}
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.exists == nil {
		deps.exists = exists
	}
	if deps.run == nil {
		deps.run = func(dir, label string, timeout time.Duration, stdin string, name string, args ...string) verifycontract.StepResult {
			return verification.Run(dir, label, timeout, stdin, commandOutputBudgetBytes, name, args...)
		}
	}
	return deps
}

func Validate(binary, root string) verifycontract.StepResult {
	return validateCommandPolicyWithDeps(binary, root, commandPolicyValidationDeps{})
}

func validateCommandPolicy(binary, root string) verifycontract.StepResult {
	return Validate(binary, root)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func validateCommandPolicyWithDeps(binary, root string, deps commandPolicyValidationDeps) verifycontract.StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempWorkspace, err := deps.makeTempDir("workspace")
	if err != nil {
		return verifydomain.FailedStep("command policy smoke", err)
	}
	defer func() { _ = deps.removeAll(tempWorkspace) }()
	outside, err := deps.makeTempDir("outside")
	if err != nil {
		return verifydomain.FailedStep("command policy smoke", err)
	}
	defer func() { _ = deps.removeAll(outside) }()

	stdoutParts := []string{}
	commands := []string{}
	for _, check := range commandPolicyChecks(binary, tempWorkspace, outside) {
		step := deps.run(root, check.label, 30*time.Second, "", check.name, check.args...)
		stdoutParts = append(stdoutParts, step.Stdout)
		commands = append(commands, step.Command)
		if !step.OK {
			return verifydomain.CombineFailedStep("command policy smoke", time.Since(started).Milliseconds(), step, stdoutParts, commands, aggregateOutputBudgetBytes)
		}
		if errs := check.validate(step.Stdout); len(errs) > 0 {
			return verifydomain.AssertionStepWithOutput("command policy smoke", time.Since(started).Milliseconds(), errs, stdoutParts, commands, aggregateOutputBudgetBytes)
		}
	}
	marker := filepath.Join(tempWorkspace, "marker")
	if deps.exists(marker) {
		return verifydomain.AssertionStepWithOutput("command policy smoke", time.Since(started).Milliseconds(), []string{"fake-run created marker; command executed unexpectedly"}, stdoutParts, commands, aggregateOutputBudgetBytes)
	}

	stdoutText, stdoutTruncated, stdoutBytes := verifydomain.TailWithBudget(strings.Join(stdoutParts, "\n"), aggregateOutputBudgetBytes)
	return verifycontract.StepResult{
		Label:           "command policy smoke",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      time.Since(started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}
