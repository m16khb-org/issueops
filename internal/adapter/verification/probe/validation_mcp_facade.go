package probe

import (
	"issueops/internal/adapter/verification/probe/mcpsmoke"
	selfverify "issueops/internal/contract/selfverify"
)

func ValidateMCP(binary, root string) selfverify.StepResult {
	return mcpsmoke.ValidateMCP(binary, root)
}
