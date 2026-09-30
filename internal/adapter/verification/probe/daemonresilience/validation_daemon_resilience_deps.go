package daemonresilience

import (
	selfverify "issueops/internal/contract/selfverify"

	"fmt"
	"os"
	"time"

	verification "issueops/internal/adapter/verification"
	verifydomain "issueops/internal/domain/selfverify"
)

type daemonResilienceValidationDeps struct {
	mkdirTemp func(string, string) (string, error)
	removeAll func(string) error
	writeFile func(string, []byte, os.FileMode) error
	chtimes   func(string, time.Time, time.Time) error
	stat      func(string) (os.FileInfo, error)
	exists    func(string) bool
	run       daemonResilienceCommandRunner
}

func (deps daemonResilienceValidationDeps) withDefaults() daemonResilienceValidationDeps {
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.writeFile == nil {
		deps.writeFile = os.WriteFile
	}
	if deps.chtimes == nil {
		deps.chtimes = os.Chtimes
	}
	if deps.stat == nil {
		deps.stat = os.Stat
	}
	if deps.exists == nil {
		deps.exists = exists
	}
	if deps.run == nil {
		deps.run = runDaemonResilienceCommand
	}
	return deps
}

func runDaemonResilienceCommand(root, label string, timeout time.Duration, input string, env []string, command ...string) selfverify.StepResult {
	if len(command) == 0 {
		return verifydomain.FailedStep(label, fmt.Errorf("missing command"))
	}
	return verification.RunEnv(root, label, timeout, input, env, commandOutputBudgetBytes, command[0], command[1:]...)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
