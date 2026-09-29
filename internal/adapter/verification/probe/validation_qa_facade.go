package probe

import "issueops/internal/adapter/verification/probe/qagate"

func validateRedactionAudit(root string) StepResult {
	return qagate.ValidateRedactionAudit(root)
}

func validateQAGate(root string) StepResult {
	return qagate.ValidateQAGate(root)
}

func validateMermaidDocs(root string) []string {
	return qagate.ValidateMermaidDocs(root)
}

func lintMermaidBlocks(relPath, text string) []string {
	return qagate.LintMermaidBlocks(relPath, text)
}

func findUnredactedSecretLike(text string) []string {
	return qagate.FindUnredactedSecretLike(text)
}
