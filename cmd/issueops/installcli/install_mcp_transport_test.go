package installcli

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"issueops/internal/adapter/install"
	"issueops/internal/contract/mcpservice"
	"issueops/internal/port"
	activationport "issueops/internal/port/nativeactivation"
)

type mcpServiceSetupFixture struct {
	calls     []string
	ensureErr error
}

func (f *mcpServiceSetupFixture) ReadBearer() (string, error) {
	f.calls = append(f.calls, "read")
	return "existing-bearer", nil
}

func (f *mcpServiceSetupFixture) Prepare(context.Context) (string, error) {
	f.calls = append(f.calls, "prepare")
	return "prepared-bearer", nil
}

func (f *mcpServiceSetupFixture) EnsureRunning(_ context.Context, binary string) (mcpservice.Status, error) {
	f.calls = append(f.calls, "ensure:"+filepath.Base(binary))
	if f.ensureErr != nil {
		return mcpservice.Status{Status: mcpservice.StatusConflict, ErrorCode: "port_in_use"}, f.ensureErr
	}
	return mcpservice.Status{OK: true, Status: mcpservice.StatusRunning, PID: 9, BuildID: "b"}, nil
}

func transportCommand(t *testing.T, service *mcpServiceSetupFixture, seen *[]port.NativeInstallRequest) Command {
	t.Helper()
	home := t.TempDir()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	stableRoot := installCommandTestStableRoot(t, root)
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	return testInstallCommand(t, Deps{
		IssueOpsRoot:         func() string { return root },
		ExecutablePath:       func() (string, error) { return filepath.Join(stableRoot, "bin", "issueops"), nil },
		NativeInstallRequest: install.DefaultNativeInstallRequest,
		InstallNative: func(req port.NativeInstallRequest) (port.NativeInstallResult, error) {
			*seen = append(*seen, req)
			return port.NativeInstallResult{OK: true, DryRun: req.DryRun}, nil
		},
		ActivationReadback:  func(port.NativeInstallRequest) activationport.ReadbackVerifier { return activationReadbackFixture{} },
		ActivationBackend:   &activationBackendFixture{},
		DefaultMCPTransport: "http",
		MCPURL:              "http://127.0.0.1:47831/mcp",
		MCPService:          service,
	})
}

func TestInstallHTTPDryRunOnlyReadsCredential(t *testing.T) {
	service := &mcpServiceSetupFixture{}
	var seen []port.NativeInstallRequest
	cli := transportCommand(t, service, &seen)
	if _, _, err := captureInstallCommandOutput(t, nil, func() error {
		return cli.RunInstall([]string{"--dry-run", "--json", "--path-mode=skip"})
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(service.calls, []string{"read"}) {
		t.Fatalf("dry-run service calls = %v", service.calls)
	}
	if len(seen) == 0 || seen[0].MCPTransport != "http" || seen[0].MCPURL != "http://127.0.0.1:47831/mcp" || seen[0].MCPBearer != "existing-bearer" {
		t.Fatalf("dry-run request = %+v", seen)
	}
}

func TestInstallHTTPBeginPreparesCredentialAndUnitWithoutStartingService(t *testing.T) {
	service := &mcpServiceSetupFixture{}
	var seen []port.NativeInstallRequest
	cli := transportCommand(t, service, &seen)
	t.Setenv("ISSUEOPS_NATIVE_ACTIVATION_STEP", "begin")
	if _, _, err := captureInstallCommandOutput(t, nil, func() error {
		return cli.RunInstall([]string{"--json", "--path-mode=skip"})
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(service.calls, []string{"prepare"}) {
		t.Fatalf("begin service calls = %v", service.calls)
	}
	for _, req := range seen {
		if !req.DryRun {
			t.Fatalf("begin wrote host configs: %+v", req)
		}
	}
}

func TestInstallHTTPRefusesHostMergeWhenServiceIsNotReady(t *testing.T) {
	service := &mcpServiceSetupFixture{ensureErr: errors.New("port taken")}
	var seen []port.NativeInstallRequest
	cli := transportCommand(t, service, &seen)
	t.Setenv("ISSUEOPS_NATIVE_ACTIVATION_STEP", "seal")
	t.Setenv("ISSUEOPS_NATIVE_ACTIVATION_TRANSITION_ID", validTransitionID)
	_, _, err := captureInstallCommandOutput(t, nil, func() error {
		return cli.RunInstall([]string{"--json", "--path-mode=skip"})
	})
	if err == nil || !strings.Contains(err.Error(), "host MCP configs were not changed") {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(service.calls, []string{"prepare", "ensure:issueops"}) {
		t.Fatalf("seal service calls = %v", service.calls)
	}
	for _, req := range seen {
		if !req.DryRun {
			t.Fatalf("host installer wrote configs despite a failed service check: %+v", seen)
		}
	}
}

func TestInstallHTTPValidatesTheHostPlanBeforeTouchingTheService(t *testing.T) {
	service := &mcpServiceSetupFixture{}
	var seen []port.NativeInstallRequest
	cli := transportCommand(t, service, &seen)
	cli.InstallNative = func(req port.NativeInstallRequest) (port.NativeInstallResult, error) {
		seen = append(seen, req)
		return port.NativeInstallResult{OK: false, DryRun: req.DryRun}, errors.New("open root/skills: no such file or directory")
	}
	_, _, err := captureInstallCommandOutput(t, nil, func() error {
		return cli.RunInstall([]string{"--json", "--path-mode=skip"})
	})
	if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
		t.Fatalf("err = %v", err)
	}
	if len(service.calls) != 0 {
		t.Fatalf("a failed host plan still changed the mcp service: %v", service.calls)
	}
	if len(seen) != 1 || !seen[0].DryRun || seen[0].MCPTransport != "http" {
		t.Fatalf("host plan requests = %+v", seen)
	}
}

func TestInstallStdioTransportNeverTouchesService(t *testing.T) {
	service := &mcpServiceSetupFixture{}
	var seen []port.NativeInstallRequest
	cli := transportCommand(t, service, &seen)
	if _, _, err := captureInstallCommandOutput(t, nil, func() error {
		return cli.RunInstall([]string{"--dry-run", "--json", "--path-mode=skip", "--mcp-transport=stdio"})
	}); err != nil {
		t.Fatal(err)
	}
	if len(service.calls) != 0 || len(seen) == 0 || seen[0].MCPTransport != "stdio" || seen[0].MCPBearer != "" {
		t.Fatalf("calls=%v seen=%+v", service.calls, seen)
	}
	if err := cli.RunInstall([]string{"--dry-run", "--mcp-transport=sse"}); err == nil {
		t.Fatal("unknown transport accepted")
	}
}
