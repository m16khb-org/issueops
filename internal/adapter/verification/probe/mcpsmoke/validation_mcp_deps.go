package mcpsmoke

import (
	"os"
	"strings"
	"time"

	verifycontract "issueops/internal/contract/selfverify"
)

const aggregateOutputBudgetBytes = 8 * 1024

// MCPValidationDeps are the effects of the MCP smoke check: temp directory
// creation and removal, and the SDK smoke run. Nil fields default to the real
// implementations.
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

func splitLines(s string) []string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}
