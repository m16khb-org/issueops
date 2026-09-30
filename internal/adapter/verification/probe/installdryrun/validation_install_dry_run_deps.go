package installdryrun

import (
	selfverify "issueops/internal/contract/selfverify"

	"fmt"
	"os"
	"time"

	verification "issueops/internal/adapter/verification"
)

type installDryRunCommandRunner func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) selfverify.StepResult

type installDryRunValidationDeps struct {
	makeTempDir func(kind string, seed int64) (string, error)
	removeAll   func(path string) error
	makeDirAll  func(path string, perm uint32) error
	writeFile   func(path string, data []byte, perm uint32) error
	exists      func(path string) bool
	run         installDryRunCommandRunner
}

func (deps installDryRunValidationDeps) withDefaults() installDryRunValidationDeps {
	if deps.makeTempDir == nil {
		deps.makeTempDir = func(kind string, seed int64) (string, error) {
			return os.MkdirTemp("", fmt.Sprintf("issueops-install-%s-%d-*", kind, seed))
		}
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.makeDirAll == nil {
		deps.makeDirAll = func(path string, perm uint32) error {
			return os.MkdirAll(path, os.FileMode(perm))
		}
	}
	if deps.writeFile == nil {
		deps.writeFile = func(path string, data []byte, perm uint32) error {
			return os.WriteFile(path, data, os.FileMode(perm))
		}
	}
	if deps.exists == nil {
		deps.exists = exists
	}
	if deps.run == nil {
		deps.run = func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) selfverify.StepResult {
			return verification.RunEnv(dir, label, timeout, stdin, env, commandOutputBudgetBytes, name, args...)
		}
	}
	return deps
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
