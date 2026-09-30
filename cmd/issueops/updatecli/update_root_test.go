package updatecli

import (
	adapter "issueops/internal/adapter/update"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateInstallerUsesRequestedRootAsWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", outside)
	script := filepath.Join(root, "scripts", "install-native.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' \"$PWD\" > \"$0.cwd\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := (adapter.Runtime{Environment: []string{"PATH=/usr/bin:/bin"}}).Install(root, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(script + ".cwd")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := filepath.EvalSymlinks(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if observed != expected {
		t.Fatalf("installer used ambient root: got %q, want %q", strings.TrimSpace(string(raw)), expected)
	}
}
