//go:build darwin || linux

package cmux

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadPromptAcceptsEmptyMode0600File(t *testing.T) {
	root := canonicalTempDir(t)
	path := filepath.Join(root, "prompt")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPrompt(root, path, digestBytes(nil))
	if err != nil || len(got) != 0 {
		t.Fatalf("prompt=%q err=%v", got, err)
	}
}

func TestReadPromptRejectsLeadingNULWithNULError(t *testing.T) {
	root := canonicalTempDir(t)
	path := filepath.Join(root, "prompt")
	value := []byte("\x00sealed")
	if err := os.WriteFile(path, value, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPrompt(root, path, digestBytes(value)); err == nil || err.Error() != "cmux prompt must not contain NUL bytes" {
		t.Fatalf("leading NUL error=%v", err)
	}
}
