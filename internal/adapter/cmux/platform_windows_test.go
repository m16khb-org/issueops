//go:build windows

package cmux

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsRefusesCmuxEndpointIdentityAndPromptDelivery(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, _, err := fileIdentity(info); !errors.Is(err, errUnsupportedPlatform) {
		t.Fatalf("fileIdentity err=%v want the unsupported-platform refusal", err)
	}
	if _, err := readPromptPlatform(filepath.Dir(file), file, "", nil); !errors.Is(err, errUnsupportedPlatform) {
		t.Fatalf("readPromptPlatform err=%v want the unsupported-platform refusal", err)
	}
}
