// Package verifywork is the application service that verifies a unit of work
// by combining git status, preflight, guard and command evidence into one result.
package verifywork

import (
	guardcontract "issueops/internal/contract/guard"
	policycontract "issueops/internal/contract/policy"
	preflightcontract "issueops/internal/contract/preflight"
	verifyworkcontract "issueops/internal/contract/verifywork"
	projectdocdomain "issueops/internal/domain/projectdoc"
	verifyworkdomain "issueops/internal/domain/verifywork"
)

// Service verifies a unit of work in a repository by gathering git status,
// preflight, guard and optional command results. Every function field is required.
type Service struct {
	ResolveTarget  func(string) string
	GitStatus      func(string) (string, error)
	Preflight      func(string) preflightcontract.Result
	Guard          func(guardcontract.CheckRequest) guardcontract.CheckResult
	RunCommand     func(policycontract.CommandPolicyRequest) policycontract.CommandRunResult
	ProjectSignals func(string) projectdocdomain.ProjectSignals
}

// Run gathers git status, preflight and guard results for repo (all changes
// when all is true, otherwise staged ones), optionally runs argv under the
// command policy with a 30s timeout, and returns the evidence-backed verdict.
// Failures of the individual checks are reported in the result, not as errors.
func (service Service) Run(repo string, all bool, argv []string) verifyworkcontract.Result {
	root := service.ResolveTarget(repo)
	status, err := service.GitStatus(root)
	facts := verifyworkdomain.Observation{Argv: argv}
	if err != nil {
		facts.GitFailed = true
		facts.GitError = err.Error()
	}
	preflight := service.Preflight(root)
	guard := service.Guard(guardcontract.CheckRequest{RepoRoot: root, Staged: !all, All: all})
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
