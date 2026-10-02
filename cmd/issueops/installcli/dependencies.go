package installcli

import (
	"context"
	"encoding/json"
	"os"

	installcontract "issueops/internal/contract/install"
	upstreamcontract "issueops/internal/contract/upstream"
	"issueops/internal/port"
	activationport "issueops/internal/port/nativeactivation"
)

// Command keeps one installation configuration through all transaction phases.
type Command struct{ Deps }

type Deps struct {
	StateRoot                          string
	EnsureSymlinkPlan                  func(target, path string, dryRun bool) (port.InstallLink, error)
	PrepareManagedCommandPathCandidate func(target, candidate, path string, adopt, dryRun bool) (ManagedCommandPathTransaction, installcontract.ManagedCommandPathPlan, error)

	IssueOpsRoot      func() string
	ActivationBackend activationport.Backend

	// ExecutablePath는 현재 실행 파일 경로를 돌려준다. managed command file을
	// 안전하게 채택하려면 후보가 자기 자신인지 가려야 한다.
	ExecutablePath func() (string, error)

	// NativeInstallRequest와 InstallNative는 composition root가 주입한다.
	// 어떤 host installer를 조립할지는 CLI가 알 필요가 없다.
	NativeInstallRequest func(root, home, codexHome, binPath string) port.NativeInstallRequest
	InstallNative        func(port.NativeInstallRequest) (port.NativeInstallResult, error)

	// ActivationReadback은 host별 활성화 증적을 모으는 verifier를 만든다.
	// 어떤 host adapter를 조립할지는 composition root가 정한다.
	ActivationReadback func(port.NativeInstallRequest) activationport.ReadbackVerifier

	// SyncUpstream은 선언된 upstream plugin/skill 중 host에 없는 것만 설치한다.
	// 미주입이면 install은 upstream을 건드리지 않고 그대로 진행한다.
	SyncUpstream func(ctx context.Context, root string, dryRun bool) (upstreamcontract.Report, error)

	// DefaultMCPTransport applies when --mcp-transport is omitted; empty means stdio.
	DefaultMCPTransport string
	MCPURL              string
	MCPService          MCPServiceSetup
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
