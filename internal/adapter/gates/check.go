// Package gates는 태스크 게이트 ledger의 파일 I/O와 policy 게이트 실행을 소유한다.
//
// unlazy gate-check.mjs의 실행 의미를 그대로 따르되, CHECK 명령은 raw shell이
// 아니라 command policy 경로로 실행한다: argv 토큰화, workspace 경계, env
// allowlist, secret redaction, timeout, audit log. 통과에는 exit code 0과
// EXPECT가 있을 때의 출력 일치가 모두 필요하다.
package gates

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gatesapp "issueops/internal/application/gates"
	gatescontract "issueops/internal/contract/gates"
	policycontract "issueops/internal/contract/policy"
	gatesdomain "issueops/internal/domain/gates"
)

// ErrUnmetGates는 게이트가 아직 미충족일 때의 결과 오류이다(unlazy exit 1).
type ErrUnmetGates struct {
	Unmet int
}

func (e ErrUnmetGates) Error() string {
	return fmt.Sprintf("%d unmet gate(s) remain", e.Unmet)
}

// Check는 게이트 파일들을 평가하고, StatusOnly가 아니면 미충족 게이트의 CHECK
// 명령을 실행해 체크박스와 증거를 파일에 기록한다.
func Check(req gatescontract.CheckRequest) (gatescontract.CheckResult, error) {
	return (gatesapp.Service{Store: ledgerFileStore{}, Runner: policyGateRunner{}, Clock: gateClock{}}).Check(req)
}

type ledgerFileStore struct{}

func (ledgerFileStore) Discover(cwd string) ([]string, error) { return DiscoverGateFiles(cwd) }
func (ledgerFileStore) Read(file string) ([]byte, error)      { return os.ReadFile(file) }
func (ledgerFileStore) WritePreservingMode(file string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(file, data, mode)
}

type gateClock struct{}

func (gateClock) Now() time.Time { return time.Now() }

type policyGateRunner struct{}

func (policyGateRunner) Run(root, cwd string, req gatescontract.CheckRequest, gate gatesdomain.Gate) gatesapp.Outcome {
	return runGateCheck(root, cwd, req, gate)
}

func runGateCheck(root, cwd string, req gatescontract.CheckRequest, gate gatesdomain.Gate) gatesapp.Outcome {
	outcome := gatesapp.Outcome{}
	argv, checkError := gatesdomain.CheckArgv(gate.CheckCmd)
	if checkError != "" {
		outcome.CheckError = checkError
		return outcome
	}
	policyReq := policycontract.CommandPolicyRequest{
		WorkspaceRoot:  root,
		CWD:            cwd,
		Argv:           argv,
		Timeout:        fmt.Sprintf("%ds", req.TimeoutSeconds),
		EnvAllowlist:   req.EnvAllowlist,
		WriteAllowed:   req.WriteAllowed,
		NetworkAllowed: req.NetworkAllowed,
	}
	evaluation := EvaluateCommandPolicy(policyReq)
	if !evaluation.Allowed {
		outcome.PolicyDenied = true
		outcome.AuditLogID = evaluation.AuditLogID
		outcome.CheckError = "check denied by policy: " + strings.Join(evaluation.DenyReasons, "; ")
		return outcome
	}
	run := RunCommand(policyReq)
	outcome.AuditLogID = run.Policy.AuditLogID
	decision := gatesdomain.DecideCheck(gate.Expect, run.Stdout, run.Stderr, run.ExitCode, run.TimedOut)
	outcome.Passed = decision.Passed
	outcome.CheckError = decision.Error
	if decision.Passed {
		outcome.Evidence = decision.Evidence
	}
	return outcome
}

// IssueFolderDir은 이슈 번호별 산출물 폴더의 상대 경로다(#480). 게이트
// 원장은 그 안의 gates.md 하나다.
const IssueFolderDir = ".issueops/issues"

// DiscoverGateFiles는 canonical .issueops/issues/<n>/gates.md를 먼저
// 찾고(번호 오름차순, 그다음 비숫자 폴더), 기존 root GATES.md,
// .issueops/gates/*.md, gates/*.md도 읽기 호환 경로로 반환한다.
func DiscoverGateFiles(root string) ([]string, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}
	files := appendIssueFolderGateFiles([]string{}, filepath.Join(root, filepath.FromSlash(IssueFolderDir)))
	if info, err := os.Stat(filepath.Join(root, "GATES.md")); err == nil && !info.IsDir() {
		files = append(files, filepath.Join(root, "GATES.md"))
	}
	files = appendMarkdownGateFiles(files, filepath.Join(root, ".issueops", "gates"))
	files = appendMarkdownGateFiles(files, filepath.Join(root, "gates"))
	return files, nil
}

// appendIssueFolderGateFiles는 issues/<name>/gates.md만 후보로 넣는다. 같은
// 폴더의 plan.md/spec.md는 원장이 아니다.
func appendIssueFolderGateFiles(files []string, dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	names := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, entry.Name(), "gates.md")); err == nil && !info.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.SliceStable(names, func(i, j int) bool {
		ni, oki := issueFolderNumber(names[i])
		nj, okj := issueFolderNumber(names[j])
		switch {
		case oki && okj:
			return ni < nj
		case oki != okj:
			return oki
		default:
			return names[i] < names[j]
		}
	})
	for _, name := range names {
		files = append(files, filepath.Join(dir, name, "gates.md"))
	}
	return files
}

func issueFolderNumber(name string) (int, bool) {
	if name == "" {
		return 0, false
	}
	n := 0
	for _, r := range name {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

func appendMarkdownGateFiles(files []string, dir string) []string {
	entries, err := os.ReadDir(dir)
	if err == nil {
		names := []string{}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				names = append(names, entry.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			files = append(files, filepath.Join(dir, name))
		}
	}
	return files
}
