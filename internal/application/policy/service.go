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

type OverrideSnapshot struct {
	Values  *policycontract.PolicyOverrides
	Warning string
}

type OverrideLoader interface {
	Load(root string) OverrideSnapshot
}

type Service struct {
	Observer  Observer
	Overrides OverrideLoader
	Executor  Executor
	Clock     Clock
}

func (service Service) Evaluate(request policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation {
	observation := service.Observer.Observe(request)
	facts := observation.Facts
	catalog := policydomain.BuiltinCatalog()
	if service.Overrides != nil {
		overrides := service.Overrides.Load(observation.Root)
		if overrides.Values != nil {
			catalog.Apply(*overrides.Values)
		}
		if overrides.Warning != "" {
			facts.Warnings = append(facts.Warnings, overrides.Warning)
		}
	}
	if len(request.Argv) > 0 {
		classification := catalog.Classify(request.Argv)
		facts.ShellCommand = classification.ShellCommand
		facts.UsesNetwork = classification.UsesNetwork
		facts.Writes = classification.Writes
		facts.ReadOnlyAllowed = classification.ReadOnlyAllowed
	}
	decision := policydomain.EvaluateCommandDecision(request, facts)
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
