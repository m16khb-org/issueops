package issueopsapp

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"issueops/cmd/issueops/mcpcli"
)

func TestSupervisorUnitRunsAbsoluteBinaryWithExplicitRootAndState(t *testing.T) {
	var unitPath string
	home := t.TempDir()
	switch runtime.GOOS {
	case "darwin":
		unitPath = filepath.Join(home, "Library", "LaunchAgents", "io.issueops.unit-test.plist")
	case "linux":
		unitPath = filepath.Join(home, ".config", "systemd", "user", "io.issueops.unit-test.service")
	default:
		t.Skip("no supervisor on " + runtime.GOOS)
	}
	state := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ISSUEOPS_STATE_DIR", state)
	t.Setenv("ISSUEOPS_ROOT", root)
	t.Setenv(mcpServiceLabelEnv, "io.issueops.unit-test")

	bearer, err := newSupervisorMCPService().Prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	stored, err := mcpcli.EnsureHTTPBearer(state)
	if err != nil || stored != bearer {
		t.Fatalf("prepared bearer does not match the state credential: %v", err)
	}
	raw, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatal(err)
	}
	unit := string(raw)
	for _, want := range []string{filepath.Join(root, "bin", "issueops"), "mcp", "--http", "ISSUEOPS_ROOT", root, "ISSUEOPS_STATE_DIR", state} {
		if !strings.Contains(unit, want) {
			t.Fatalf("unit missing %q:\n%s", want, unit)
		}
	}
	if strings.Contains(unit, bearer) {
		t.Fatal("unit must not contain the bearer")
	}
	status, err := newSupervisorMCPService().Stop(t.Context())
	if status.URL != "http://"+mcpcli.DefaultHTTPAddress+mcpcli.HTTPEndpointPath {
		t.Fatalf("service url = %q (err %v)", status.URL, err)
	}
}
