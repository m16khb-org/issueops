package mcpsmoke

import (
	"os"
	"strings"
	"time"

	verifycontract "issueops/internal/contract/selfverify"
	verifydomain "issueops/internal/domain/selfverify"
)

const aggregateOutputBudgetBytes = 8 * 1024

type MCPValidationDeps struct {
	MkdirTemp   func(string, string) (string, error)
	RemoveAll   func(string) error
	RunSDKSmoke func(string, string, []string, time.Duration) verifycontract.StepResult
}

func (deps MCPValidationDeps) withDefaults() MCPValidationDeps {
	if deps.MkdirTemp == nil {
		deps.MkdirTemp = os.MkdirTemp
	}
	if deps.RemoveAll == nil {
		deps.RemoveAll = os.RemoveAll
	}
	if deps.RunSDKSmoke == nil {
		deps.RunSDKSmoke = runSDKSmoke
	}
	return deps
}

func failedStep(label string, err error) verifycontract.StepResult {
	return verifydomain.FailedStep(label, err)
}

func tailWithBudget(s string, max int) (string, bool, int) {
	return verifydomain.TailWithBudget(s, max)
}

func splitLines(s string) []string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}
