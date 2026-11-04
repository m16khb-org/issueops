// Package mcpsmoke runs the MCP smoke check of the self-verification loop
// against a built issueops binary. It is an adapter-layer verification probe.
package mcpsmoke

import (
	selfverify "issueops/internal/contract/selfverify"
	verifydomain "issueops/internal/domain/selfverify"
	"time"
)

// ValidateMCP runs the MCP smoke check of binary under root with real
// temp-dir and SDK smoke dependencies.
func ValidateMCP(binary, root string) selfverify.StepResult {
	return ValidateMCPWithDeps(binary, root, MCPValidationDeps{})
}

// ValidateMCPWithDeps runs the MCP smoke check of binary under root, with a
// throwaway ISSUEOPS_STATE_DIR, a 30 second SDK smoke run, and the MCP smoke
// contract check. Nil fields in deps fall back to the real implementations.
// Output is trimmed to the aggregate output budget.
func ValidateMCPWithDeps(binary, root string, deps MCPValidationDeps) selfverify.StepResult {
	deps = deps.withDefaults()
	tempState, err := deps.MkdirTemp("", "issueops-mcp-state-*")
	if err != nil {
		return verifydomain.FailedStep("MCP smoke", err)
	}
	defer func() { _ = deps.RemoveAll(tempState) }()
	env := []string{"ISSUEOPS_STATE_DIR=" + tempState}

	step := deps.RunSDKSmoke(root, binary, env, 30*time.Second)
	if !step.OK {
		return step
	}
	validateMCPSmokeContract(&step)
	step.Stdout, step.StdoutTruncated, step.StdoutBytes = verifydomain.TailWithBudget(step.Stdout, aggregateOutputBudgetBytes)
	return step
}
