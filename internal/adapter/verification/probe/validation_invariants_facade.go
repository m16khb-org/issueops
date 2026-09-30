package probe

import (
	selfverify "issueops/internal/contract/selfverify"
)

import "issueops/internal/adapter/verification/probe/invariants"

func containsForbiddenLegacyOutsideRuntimePaths(text, root string) bool {
	return invariants.ContainsForbiddenLegacyOutsideRuntimePaths(text, root)
}

func forbiddenNameHits(root string) []string {
	return invariants.ForbiddenNameHits(root)
}

func validateHarnessInvariants(root string) selfverify.StepResult {
	return invariants.ValidateHarnessInvariants(root)
}
