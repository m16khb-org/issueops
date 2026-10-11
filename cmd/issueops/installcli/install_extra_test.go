package installcli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"issueops/internal/contract/mcpservice"
	"issueops/internal/port"
)

type failingMCPService struct {
	readErr, prepareErr error
}

func (f failingMCPService) ReadBearer() (string, error)             { return "", f.readErr }
func (f failingMCPService) Prepare(context.Context) (string, error) { return "", f.prepareErr }
func (f failingMCPService) EnsureRunning(context.Context, string) (mcpservice.Status, error) {
	return mcpservice.Status{OK: true}, nil
}

func TestInstallHTTPSetupFailuresAreReportedWithTheirStage(t *testing.T) {
	boom := errors.New("keychain locked")
	t.Run("dry-run credential read failure", func(t *testing.T) {
		cli := transportCommand(t, &mcpServiceSetupFixture{}, new([]port.NativeInstallRequest))
		cli.MCPService = failingMCPService{readErr: boom}
		_, _, err := captureInstallCommandOutput(t, nil, func() error {
			return cli.RunInstall([]string{"--dry-run", "--json", "--path-mode=skip"})
		})
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want %v", err, boom)
		}
	})
	t.Run("credential prepare failure", func(t *testing.T) {
		cli := transportCommand(t, &mcpServiceSetupFixture{}, new([]port.NativeInstallRequest))
		cli.MCPService = failingMCPService{prepareErr: boom}
		_, _, err := captureInstallCommandOutput(t, nil, func() error {
			return cli.RunInstall([]string{"--json", "--path-mode=skip"})
		})
		if !errors.Is(err, boom) || !strings.Contains(err.Error(), "prepare mcp http service:") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("preflight reporting ok=false without an error is still a refusal", func(t *testing.T) {
		service := &mcpServiceSetupFixture{}
		cli := transportCommand(t, service, new([]port.NativeInstallRequest))
		cli.InstallNative = func(req port.NativeInstallRequest) (port.NativeInstallResult, error) {
			return port.NativeInstallResult{OK: false, DryRun: req.DryRun}, nil
		}
		_, _, err := captureInstallCommandOutput(t, nil, func() error {
			return cli.RunInstall([]string{"--json", "--path-mode=skip"})
		})
		if err == nil || !strings.Contains(err.Error(), "reported ok=false") {
			t.Fatalf("err = %v", err)
		}
		if len(service.calls) != 0 {
			t.Fatalf("service touched despite failed preflight: %v", service.calls)
		}
	})
}

func TestInteractiveInstallRequiresTerminalStdin(t *testing.T) {
	cli := runInstallStub(t, nil)
	t.Setenv("ISSUEOPS_INSTALL_HELPER", "")
	_, _, err := captureInstallCommandOutput(t, strings.NewReader("y\n"), func() error {
		return cli.RunInstall([]string{"--interactive", "--dry-run"})
	})
	if err == nil || !strings.Contains(err.Error(), "requires a terminal stdin") {
		t.Fatalf("err = %v", err)
	}
}

func TestPrintInstallNativeResultDescribesManagedCommandAdoptionState(t *testing.T) {
	cases := []struct {
		name    string
		command port.ManagedCommandPathResult
		want    string
	}{
		{"approved by default", port.ManagedCommandPathResult{}, "(approved)"},
		{"would adopt wins", port.ManagedCommandPathResult{WouldAdopt: true, Adopted: true, Committed: true, RolledBack: true}, "(would adopt)"},
		{"rolled back beats committed", port.ManagedCommandPathResult{RolledBack: true, Committed: true, Adopted: true}, "(rolled back)"},
		{"committed beats adopted", port.ManagedCommandPathResult{Committed: true, Adopted: true}, "(committed)"},
		{"adopted awaits seal", port.ManagedCommandPathResult{Adopted: true}, "(adopted pending activation seal)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			command := tc.command
			command.Path, command.Target = "/home/u/.local/bin/issueops", "/root/bin/issueops"
			out, _, _ := captureInstallCommandOutput(t, nil, func() error {
				printInstallNativeResult(port.NativeInstallResult{Root: "/root", CommandPath: &command})
				return nil
			})
			line := "- managed command adoption: /home/u/.local/bin/issueops -> /root/bin/issueops " + tc.want
			if !strings.Contains(out, line) {
				t.Fatalf("output missing %q:\n%s", line, out)
			}
			if strings.Contains(out, "backup/recovery") {
				t.Fatalf("no backup line expected without a backup path:\n%s", out)
			}
		})
	}
	out, _, _ := captureInstallCommandOutput(t, nil, func() error {
		printInstallNativeResult(port.NativeInstallResult{CommandPath: &port.ManagedCommandPathResult{BackupPath: "/b", RollbackAvailable: true}})
		return nil
	})
	if !strings.Contains(out, "backup/recovery path: /b (rollback_available=true, backup_retained=false)") {
		t.Fatalf("backup line missing:\n%s", out)
	}
}

func TestInstallPathTransactionApplyStopsWhenCommandSymlinkFails(t *testing.T) {
	boom := errors.New("symlink denied")
	transaction := &installPathTransaction{
		req:  port.NativeInstallRequest{BinPath: "/t/issueops"},
		path: "/h/.local/bin/issueops", shortPath: "/h/.local/bin/io",
		ensureSymlinkPlan: func(target, path string, dryRun bool) (port.InstallLink, error) {
			if dryRun || path != "/h/.local/bin/issueops" {
				t.Errorf("unexpected symlink call %q %q dry=%v", target, path, dryRun)
			}
			return port.InstallLink{Path: path, Target: target}, boom
		},
	}
	var result port.NativeInstallResult

	if err := transaction.Apply(&result); !errors.Is(err, boom) {
		t.Fatalf("apply err = %v", err)
	}
	if transaction.commandCreated || transaction.shortCreated || len(result.Links) != 1 {
		t.Fatalf("nothing may be marked created after the command link failed: %+v links=%v", transaction, result.Links)
	}
}
