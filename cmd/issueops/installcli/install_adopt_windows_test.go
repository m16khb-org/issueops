//go:build windows

package installcli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/port"
	"issueops/internal/testsupport"
)

func TestInstallRefusesCommandAdoptionOnWindows(t *testing.T) {
	home := t.TempDir()
	target := buildManagedTestCommand(t)
	command := filepath.Join(home, ".local", "bin", testsupport.ExecutableName("issueops"))
	copyTestCommand(t, target, command)
	want, err := os.ReadFile(command)
	if err != nil {
		t.Fatal(err)
	}

	for _, dryRun := range []bool{true, false} {
		result := port.NativeInstallResult{}
		transaction, err := prepareInstallPathPlanForTest(t, &result, port.NativeInstallRequest{Home: home, BinPath: target, DryRun: dryRun, AdoptCommandFile: true}, "skip")
		if err == nil || !strings.Contains(err.Error(), "--adopt-command-file is not supported on windows") {
			t.Fatalf("dryRun=%v adoption error = %v", dryRun, err)
		}
		if transaction != nil {
			t.Fatalf("dryRun=%v refused adoption still returned a transaction", dryRun)
		}
		if result.CommandPath != nil && result.CommandPath.WouldAdopt {
			t.Fatalf("dryRun=%v refused adoption reported WouldAdopt: %+v", dryRun, result.CommandPath)
		}
	}
	got, err := os.ReadFile(command)
	info, statErr := os.Lstat(command)
	if err != nil || statErr != nil || !bytes.Equal(got, want) || !info.Mode().IsRegular() {
		t.Fatalf("refused adoption changed the command: info=%v err=%v statErr=%v", info, err, statErr)
	}
}
