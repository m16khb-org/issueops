package mcpsmoke

import (
	selfverify "issueops/internal/contract/selfverify"
	verifydomain "issueops/internal/domain/selfverify"
	"time"
)

func ValidateMCP(binary, root string) selfverify.StepResult {
	return ValidateMCPWithDeps(binary, root, MCPValidationDeps{})
}

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
	ValidateMCPSmokeContract(&step)
	step.Stdout, step.StdoutTruncated, step.StdoutBytes = verifydomain.TailWithBudget(step.Stdout, aggregateOutputBudgetBytes)
	return step
}
