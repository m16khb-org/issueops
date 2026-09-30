package probe

import "issueops/internal/adapter/verification/probe/qagate"

func lintMermaidBlocks(relPath, text string) []string {
	return qagate.LintMermaidBlocks(relPath, text)
}

func findUnredactedSecretLike(text string) []string {
	return qagate.FindUnredactedSecretLike(text)
}
