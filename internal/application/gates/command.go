package gates

import (
	"fmt"
	model "issueops/internal/contract/gates"
	policy "issueops/internal/contract/policy"
	domain "issueops/internal/domain/gates"
)

type CommandRunner struct {
	Evaluate func(policy.CommandPolicyRequest) policy.CommandPolicyEvaluation
	Execute  func(policy.CommandPolicyRequest) policy.CommandRunResult
}

func (runner CommandRunner) Run(root, cwd string, req model.CheckRequest, gate domain.Gate) domain.CheckOutcome {
	argv, checkError := domain.CheckArgv(gate.CheckCmd)
	if checkError != "" {
		return domain.CheckOutcome{CheckError: checkError}
	}
	request := policy.CommandPolicyRequest{WorkspaceRoot: root, CWD: cwd, Argv: argv, Timeout: fmt.Sprintf("%ds", req.TimeoutSeconds), EnvAllowlist: req.EnvAllowlist, WriteAllowed: req.WriteAllowed, NetworkAllowed: req.NetworkAllowed}
	evaluation := runner.Evaluate(request)
	if !evaluation.Allowed {
		return domain.DeniedCheck(evaluation.AuditLogID, evaluation.DenyReasons)
	}
	run := runner.Execute(request)
	return domain.CompletedCheck(gate, run.Stdout, run.Stderr, run.ExitCode, run.TimedOut, run.Policy.AuditLogID)
}
