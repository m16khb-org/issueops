//go:build windows

package installutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareManagedCommandPathRefusesAdoptionOnWindows(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.exe")
	command := filepath.Join(root, "issueops.exe")
	for _, path := range []string{target, command} {
		if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if _, _, err := PrepareManagedCommandPathCandidate(target, target, command, false, false); err == nil || !strings.Contains(err.Error(), "refusing to adopt regular command file without --adopt-command-file") {
		t.Fatalf("unapproved adoption error = %v", err)
	}
	for _, dryRun := range []bool{true, false} {
		transaction, plan, err := PrepareManagedCommandPathCandidate(target, target, command, true, dryRun)
		if err == nil || !strings.Contains(err.Error(), "--adopt-command-file is not supported on windows") {
			t.Fatalf("dryRun=%v error = %v", dryRun, err)
		}
		if transaction != nil || plan.WouldAdopt || plan.BackupPath != "" {
			t.Fatalf("dryRun=%v transaction=%v plan=%+v", dryRun, transaction, plan)
		}
	}
	if data, err := os.ReadFile(command); err != nil || string(data) != "x" {
		t.Fatalf("command changed: %q %v", data, err)
	}
}
