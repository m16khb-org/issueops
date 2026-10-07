package verifywork

import (
	guardcontract "issueops/internal/contract/guard"
	policycontract "issueops/internal/contract/policy"
	preflightcontract "issueops/internal/contract/preflight"
	verifyworkcontract "issueops/internal/contract/verifywork"
	projectdocdomain "issueops/internal/domain/projectdoc"
	verifyworkdomain "issueops/internal/domain/verifywork"
)

type Service struct {
	ResolveTarget  func(string) string
	GitStatus      func(string) (string, error)
	Preflight      func(string) preflightcontract.PreflightResult
	Guard          func(guardcontract.GuardCheckRequest) guardcontract.GuardCheckResult
	RunCommand     func(policycontract.CommandPolicyRequest) policycontract.CommandRunResult
	ProjectSignals func(string) projectdocdomain.ProjectSignals
}

func (service Service) Run(repo string, all bool, argv []string) verifyworkcontract.Result {
	root := service.ResolveTarget(repo)
	status, err := service.GitStatus(root)
	facts := verifyworkdomain.Observation{Argv: argv}
	if err != nil {
		facts.GitFailed = true
		facts.GitError = err.Error()
	}
	preflight := service.Preflight(root)
	guard := service.Guard(guardcontract.GuardCheckRequest{RepoRoot: root, Staged: !all, All: all})
	var command *policycontract.CommandRunResult
	if len(argv) > 0 {
		result := service.RunCommand(policycontract.CommandPolicyRequest{WorkspaceRoot: root, CWD: root, Argv: argv, Timeout: "30s"})
		command = &result
		facts.CommandPresent = true
		facts.CommandOK = result.OK
	}
	facts.Signals = service.ProjectSignals(root)
	facts.PreflightOK = preflight.OK
	facts.GuardOK = guard.OK
	facts.GuardMode = guard.Mode
	facts.GuardFileCount = len(guard.CheckedFiles)
	decision := verifyworkdomain.Evaluate(facts)
	return verifyworkcontract.Result{
		OK: decision.OK, Kind: "verify_work", Repo: root, GitStatus: status,
		Preflight: preflight, Guard: guard, Command: command,
		EvidenceMatrix:    decision.EvidenceMatrix,
		SuggestedCommands: decision.SuggestedCommands, Warnings: decision.Warnings,
	}
}
