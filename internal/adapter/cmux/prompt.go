package cmux

// ReadPrompt reads one sealed prompt through the canonical-worktree handle
// boundary. The platform implementation must not follow symlinks.
func ReadPrompt(root, path, expectedDigest string) ([]byte, error) {
	return readPromptPlatform(root, path, expectedDigest, nil)
}
