package issueopsapp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"issueops/cmd/issueops/mcpcli"
	installadapter "issueops/internal/adapter/install"
	mcpserviceadapter "issueops/internal/adapter/mcpservice"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/processinspect"
	"issueops/internal/contract/processidentity"
)

const (
	mcpServiceLaunchdLabel = "io.issueops.mcp"
	mcpServiceSystemdUnit  = "issueops-mcp.service"
	// mcpServiceLabelEnv lets isolated verification use a unique supervisor
	// job instead of the user's real io.issueops.mcp.
	mcpServiceLabelEnv = "ISSUEOPS_MCP_SERVICE_LABEL"
)

func mcpServiceStateDir() string {
	dir := statestore.StateDir()
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

func mcpServiceProcessInspector() func(int) (processidentity.Identity, error) {
	ps, err := exec.LookPath("ps")
	return processinspect.Inspector{PSExecutable: ps, PSLookupError: err, Environment: os.Environ()}.Inspect
}

func newSupervisorMCPService() *mcpserviceadapter.Service {
	native := installadapter.DefaultNativeInstallRequest(issueOpsRoot(), "", "", "")
	stateDir := mcpServiceStateDir()
	label, unit := mcpServiceLaunchdLabel, mcpServiceSystemdUnit
	if override := os.Getenv(mcpServiceLabelEnv); override != "" {
		label, unit = override, override+".service"
	}
	return mcpserviceadapter.New(mcpserviceadapter.Config{
		GOOS: runtime.GOOS, Home: native.Home, StateDir: stateDir, Root: native.Root, Binary: native.BinPath,
		Address: mcpcli.DefaultHTTPAddress, Path: mcpcli.HTTPEndpointPath,
		Label: label, UnitName: unit, UID: os.Getuid(),
		Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		},
		LookPath:     exec.LookPath,
		Inspect:      mcpServiceProcessInspector(),
		EnsureBearer: func() (string, error) { return mcpcli.EnsureHTTPBearer(stateDir) },
	})
}
