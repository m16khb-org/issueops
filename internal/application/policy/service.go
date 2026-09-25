package policy

import (
	"time"

	policycontract "issueops/internal/contract/policy"
	policydomain "issueops/internal/domain/policy"
)

type Observation struct {
	Root        string
	CWD         string
	Timeout     time.Duration
	AuditLogID  string
	GeneratedAt string
	Facts       policydomain.CommandFacts
}

type Observer interface {
	Observe(policycontract.CommandPolicyRequest) Observation
}

type Service struct {
	Observer Observer
	Executor Executor
	Clock    Clock
}

func (service Service) Evaluate(request policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation {
	observation := service.Observer.Observe(request)
	decision := policydomain.EvaluateCommandDecision(request, observation.Facts)
	return policycontract.CommandPolicyEvaluation{
		OK: true, Allowed: decision.Allowed, AuditLogID: observation.AuditLogID,
		WorkspaceRoot: observation.Root, CWD: observation.CWD,
		Argv: policydomain.RedactArgv(request.Argv), Timeout: observation.Timeout.String(),
		EnvAllowlist:   policydomain.CleanEnvAllowlist(request.EnvAllowlist),
		NetworkAllowed: request.NetworkAllowed, WriteAllowed: request.WriteAllowed,
		ShellAllowed: request.ShellAllowed, ShellReason: policydomain.RedactFreeform(request.ShellReason),
		Tier: policydomain.ResolveTier(policycontract.Request{
			WriteAllowed: request.WriteAllowed, NetworkAllowed: request.NetworkAllowed, ShellAllowed: request.ShellAllowed,
		}),
		DenyReasons: decision.DenyReasons, Warnings: decision.Warnings, GeneratedAt: observation.GeneratedAt,
	}
}
