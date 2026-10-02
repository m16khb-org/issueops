package issueopsapp

import (
	"context"
	"fmt"

	installapp "issueops/internal/application/install"
	installcontract "issueops/internal/contract/install"
	"os"
	"runtime"

	"issueops/cmd/issueops/installcli"
	"issueops/cmd/issueops/mcpcli"
	agyadapter "issueops/internal/adapter/agy"
	claudeadapter "issueops/internal/adapter/claude"
	codexadapter "issueops/internal/adapter/codex"
	"issueops/internal/adapter/install"
	"issueops/internal/adapter/installutil"
	mcpserviceadapter "issueops/internal/adapter/mcpservice"
	omoadapter "issueops/internal/adapter/omo"
	mcpcontract "issueops/internal/contract/mcp"
	"issueops/internal/contract/mcpservice"
	"issueops/internal/port"
	activationport "issueops/internal/port/nativeactivation"
)

// installDependencies는 native install CLI에 host installer를 조립해 넘긴다.
//
// 어떤 host를 설치하고 어떤 증적을 읽을지는 composition root의 결정이다.
// CLI는 flag 해석과 출력만 소유한다.
func installDependencies() installcli.Deps {
	root, stateRoot := issueOpsRoot(), issueOpsStateRoot()
	codex, claude, omo, agy := newCodexInstaller(), newClaudeInstaller(), newOmoInstaller(), newAgyInstaller()
	return installcli.Deps{
		IssueOpsRoot:      func() string { return root },
		StateRoot:         stateRoot,
		ExecutablePath:    os.Executable,
		EnsureSymlinkPlan: installutil.EnsureSymlinkPlan,
		PrepareManagedCommandPathCandidate: func(target, candidate, path string, adopt, dry bool) (installcli.ManagedCommandPathTransaction, installcontract.ManagedCommandPathPlan, error) {
			tx, plan, err := installutil.PrepareManagedCommandPathCandidate(target, candidate, path, adopt, dry)
			if tx == nil {
				return nil, plan, err
			}
			return tx, plan, err
		},
		ActivationBackend:    nativeActivationBackend(),
		NativeInstallRequest: install.DefaultNativeInstallRequest,
		InstallNative:        (installapp.Service{Environment: install.Environment{}, Installers: []port.HostInstaller{codex, claude, omo, agy}}).Install,
		ActivationReadback: func(req port.NativeInstallRequest) activationport.ReadbackVerifier {
			return hostActivationReadback{request: req, codex: codex, claude: claude, omo: omo, agy: agy}
		},
		SyncUpstream:        syncUpstream,
		DefaultMCPTransport: defaultMCPTransport(runtime.GOOS),
		MCPURL:              "http://" + mcpcli.DefaultHTTPAddress + mcpcli.HTTPEndpointPath,
		MCPService:          installMCPService{service: newSupervisorMCPService(), stateDir: mcpServiceStateDir()},
	}
}

func defaultMCPTransport(goos string) string {
	if goos == "darwin" || goos == "linux" {
		return "http"
	}
	return "stdio"
}

type installMCPService struct {
	service  *mcpserviceadapter.Service
	stateDir string
}

func (s installMCPService) ReadBearer() (string, error) {
	return mcpserviceadapter.ReadBearer(s.stateDir)
}

func (s installMCPService) Prepare(ctx context.Context) (string, error) {
	return s.service.Prepare(ctx)
}

func (s installMCPService) EnsureRunning(ctx context.Context, binary string) (mcpservice.Status, error) {
	return s.service.EnsureRunning(ctx, binary)
}

type hostActivationReadback struct {
	request port.NativeInstallRequest
	codex   codexadapter.Installer
	claude  claudeadapter.Installer
	omo     omoadapter.Installer
	agy     agyadapter.Installer
}

func (readback hostActivationReadback) Verify(_ context.Context, issueOpsRoot, targetBinary string) (activationport.Readback, error) {
	if readback.request.Root != issueOpsRoot || readback.request.BinPath != targetBinary {
		return activationport.Readback{}, fmt.Errorf("native activation readback target changed")
	}
	codexEvidence, err := readback.codex.VerifyActivation(readback.request)
	if err != nil {
		return activationport.Readback{}, err
	}
	claudeEvidence, err := readback.claude.VerifyActivation(readback.request)
	if err != nil {
		return activationport.Readback{}, err
	}
	omoEvidence, err := readback.omo.VerifyActivation(readback.request)
	if err != nil {
		return activationport.Readback{}, err
	}
	agyEvidence, err := readback.agy.VerifyActivation(readback.request)
	if err != nil {
		return activationport.Readback{}, err
	}
	tools := mcpcontract.IssueOpsBasicTools()
	if len(tools) != 1 || tools[0].Name != "issueops_execution" {
		return activationport.Readback{}, fmt.Errorf("IssueOps v1 MCP activation catalog must contain exactly issueops_execution")
	}
	catalogSHA, err := installutil.SemanticSHA256(tools)
	if err != nil {
		return activationport.Readback{}, err
	}
	evidence := append(codexEvidence, claudeEvidence...)
	evidence = append(evidence, omoEvidence...)
	evidence = append(evidence, agyEvidence...)
	result := make([]activationport.Evidence, 0, len(evidence))
	for _, item := range evidence {
		result = append(result, activationport.Evidence{
			Host: item.Host, Surface: item.Surface, Path: item.Path, SemanticSHA256: item.SemanticSHA256,
			SHA256: item.SHA256, Mode: item.Mode, Size: item.Size, Device: item.Device, Inode: item.Inode,
		})
	}
	return activationport.Readback{CatalogSHA256: catalogSHA, Evidence: result}, nil
}
