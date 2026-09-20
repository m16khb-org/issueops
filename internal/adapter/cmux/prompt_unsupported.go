//go:build !darwin && !linux

package cmux

func readPromptPlatform(root, path, expectedDigest string, afterOpen func()) ([]byte, error) {
	return nil, requireSupportedPlatform()
}
