package gates

import (
	"fmt"
	model "issueops/internal/contract/gates"
	policy "issueops/internal/contract/policy"
	domain "issueops/internal/domain/gates"
)

// CommandRunner 타입은 gate 하나의 CHECK 명령을 command policy로 평가한 뒤 실행하는 Runner 구현이다.
// Evaluate와 Execute는 호출하는 쪽이 주입한다.
type CommandRunner struct {
	Evaluate func(policy.CommandPolicyRequest) policy.CommandPolicyEvaluation
	Execute  func(policy.CommandPolicyRequest) policy.CommandRunResult
}

// Run 메서드는 gate.CheckCmd를 argv로 풀어 policy를 평가하고, 허용될 때만 실행해 CheckOutcome을 돌려준다.
// argv를 풀 수 없거나 policy가 거부하면 명령을 실행하지 않고 그 사유를 담은 결과를 돌려준다.
func (runner CommandRunner) Run(root, cwd string, req model.CheckRequest, gate domain.Gate) domain.CheckOutcome {
	argv, checkError := domain.CheckArgv(gate.CheckCmd)
	if checkError != "" {
		return domain.CheckOutcome{CheckError: checkError}
	}
	request := checkPolicyRequest(root, cwd, req, argv)
	evaluation := runner.Evaluate(request)
	if !evaluation.Allowed {
		return domain.DeniedCheck(evaluation.AuditLogID, evaluation.DenyReasons)
	}
	run := runner.Execute(request)
	return domain.CompletedCheck(gate, run.Stdout, run.Stderr, run.ExitCode, run.TimedOut, run.Policy.AuditLogID)
}

// checkPolicyRequest는 gates check가 CHECK 하나를 평가·실행할 때 쓰는 정책 요청이다.
// gates lint도 같은 요청으로 평가해야 check와 같은 판정을 미리 본다.
func checkPolicyRequest(root, cwd string, req model.CheckRequest, argv []string) policy.CommandPolicyRequest {
	return policy.CommandPolicyRequest{WorkspaceRoot: root, CWD: cwd, Argv: argv, Timeout: fmt.Sprintf("%ds", req.TimeoutSeconds), EnvAllowlist: req.EnvAllowlist, WriteAllowed: req.WriteAllowed, NetworkAllowed: req.NetworkAllowed}
}
