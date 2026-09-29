package mcpcli

import (
	"errors"
	channelcontract "issueops/internal/contract/channel"
	gatescontract "issueops/internal/contract/gates"
	inspectcontract "issueops/internal/contract/inspect"
	preflightcontract "issueops/internal/contract/preflight"
	"os"
	"path/filepath"

	"issueops/cmd/issueops/apidoc"
	app "issueops/internal/application/selfverify"
)

const skillName = "atomic-commit-push"

var Version = "dev"

var IssueOpsRoot = func() string {
	if root := os.Getenv("ISSUEOPS_ROOT"); root != "" {
		abs, err := filepath.Abs(root)
		if err == nil {
			return abs
		}
		return root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "skills")) {
			abs, err := filepath.Abs(dir)
			if err == nil {
				return abs
			}
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			abs, err := filepath.Abs(cwd)
			if err == nil {
				return abs
			}
			return cwd
		}
	}
}

var ResolveTarget = func(target string) string {
	if target != "" {
		return target
	}
	return IssueOpsRoot()
}

var ReadHarnessFile = func(parts ...string) (string, error) {
	path := filepath.Join(append([]string{IssueOpsRoot()}, parts...)...)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

var InspectHarness = func(repo string) any {
	return map[string]any{"ok": false, "error": "inspect dependency is not configured", "repo": repo}
}

var DaemonStatus = func() any {
	return map[string]any{"ok": false, "message": "daemon is not running"}
}

var CompatibilityContract = func() any {
	return map[string]any{"ok": false, "error": "compatibility contract dependency is not configured"}
}

var (
	errAPIDocReviewGateFailed = apidoc.ErrReviewGateFailed
	errAPIDocStaticGateFailed = apidoc.ErrStaticGateFailed
)

func isAPIDocReviewGateError(err error) bool {
	return errors.Is(err, errAPIDocReviewGateFailed)
}

func isAPIDocStaticGateError(err error) bool {
	return errors.Is(err, errAPIDocStaticGateFailed)
}

func isSelfVerificationGateError(err error) bool {
	return errors.Is(err, app.ErrSelfVerificationGateFailed)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// gates ledger 연산도 composition root가 설치한다. policy 게이트 실행
// (gates_check)은 주입된 adapter 함수를 통해서만 일어난다.
var (
	GatesCheck   func(gatescontract.CheckRequest) (gatescontract.CheckResult, error)
	GatesInit    func(gatescontract.InitRequest) (gatescontract.InitResult, error)
	GatesAbandon func(gatescontract.AbandonRequest) (gatescontract.AbandonResult, error)
)

// channel 연산도 composition root가 설치한다.
var (
	ChannelSend func(channelcontract.SendRequest) (channelcontract.SendResult, error)
	ChannelRecv func(channelcontract.RecvRequest) (channelcontract.RecvResult, error)
)

// GitPreflight와 ListSkills는 composition root가 설치한다. MCP tool router는
// git 실행이나 skill 디렉터리 탐색을 스스로 하지 않는다.
var GitPreflight func(target, issueOpsRoot string) preflightcontract.PreflightResult

var ListSkills func(root, skillName string) []inspectcontract.SkillInfo
