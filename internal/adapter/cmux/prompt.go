package cmux

import (
	"bytes"
	"fmt"
)

// ReadPrompt reads one sealed prompt through the canonical-worktree handle
// boundary. The platform implementation must not follow symlinks.
func ReadPrompt(root, path, expectedDigest string) ([]byte, error) {
	return readPromptPlatform(root, path, expectedDigest, nil)
}

func validatePromptBytes(value []byte, expectedDigest string) error {
	if bytes.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("cmux prompt must not contain NUL bytes")
	}
	if digest(value) != expectedDigest {
		return fmt.Errorf("cmux prompt digest mismatch")
	}
	return nil
}
