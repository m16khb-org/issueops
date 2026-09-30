package probe

import (
	selfverify "issueops/internal/contract/selfverify"
)

import "issueops/internal/adapter/verification/probe/mcpsmoke"

func ValidateMCP(binary, root string) selfverify.StepResult {
	return mcpsmoke.ValidateMCP(binary, root)
}

func ValidateMCPWithDeps(binary, root string, deps mcpsmoke.MCPValidationDeps) selfverify.StepResult {
	return mcpsmoke.ValidateMCPWithDeps(binary, root, deps)
}

func ValidateMCPSmokeContract(step *selfverify.StepResult) {
	mcpsmoke.ValidateMCPSmokeContract(step)
}

func MCPSmokeHasExpectedMarkers(stdout string) bool {
	return mcpsmoke.MCPSmokeHasExpectedMarkers(stdout)
}

func MCPSmokeExpectedMarkers() []string {
	return mcpsmoke.MCPSmokeExpectedMarkers()
}
