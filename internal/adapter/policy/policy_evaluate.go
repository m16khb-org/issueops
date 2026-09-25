package policy

import (
	"os"
	"time"

	policycontract "issueops/internal/contract/policy"
	policydomain "issueops/internal/domain/policy"

	"issueops/internal/domain/auditid"
)

func EvaluateCommandPolicy(req policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation {
	root := absOrOriginal(req.WorkspaceRoot)
	cwd := absOrOriginal(req.CWD)
	catalog := policyCatalogForWorkspace(root)
	canonicalRoot := canonicalPotentialPath(root)
	canonicalCWD := canonicalPotentialPath(cwd)
	argv := append([]string{}, req.Argv...)
	timeout, timeoutErr := time.ParseDuration(req.Timeout)
	if req.Timeout == "" {
		timeout = 30 * time.Second
	}
	auditID := req.AuditLogID
	if auditID == "" {
		auditID = auditid.Generate(req.WorkspaceRoot, req.CWD, req.Argv)
	}
	result := policycontract.CommandPolicyEvaluation{
		OK:             true,
		AuditLogID:     auditID,
		WorkspaceRoot:  root,
		CWD:            cwd,
		Argv:           policydomain.RedactArgv(argv),
		Timeout:        timeout.String(),
		EnvAllowlist:   policydomain.CleanEnvAllowlist(req.EnvAllowlist),
		NetworkAllowed: req.NetworkAllowed,
		WriteAllowed:   req.WriteAllowed,
		ShellAllowed:   req.ShellAllowed,
		ShellReason:    policydomain.RedactFreeform(req.ShellReason),
		Tier: policydomain.ResolveTier(policycontract.Request{
			WriteAllowed: req.WriteAllowed, NetworkAllowed: req.NetworkAllowed, ShellAllowed: req.ShellAllowed,
		}),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	facts := policydomain.CommandFacts{
		RootDirectory:   isDirectory(root),
		CWDDirectory:    isDirectory(cwd),
		CWDWithinRoot:   sameOrWithin(canonicalRoot, canonicalCWD),
		Timeout:         timeout,
		TimeoutValid:    timeoutErr == nil,
		Warnings:        catalog.warnings,
		PathOutsideRoot: commandReferencesOutsideWorkspace(canonicalRoot, canonicalCWD, argv),
	}
	if len(argv) > 0 {
		facts.ShellCommand = catalog.isShellCommand(argv[0])
		facts.UsesNetwork = catalog.commandUsesNetwork(argv)
		facts.Writes = catalog.commandWrites(argv)
		facts.ReadOnlyAllowed = catalog.readOnlyAllowed(argv)
		facts.PRTargetDeny, facts.PRTargetExpected = pullRequestTargetDeny(root, cwd, argv)
	}
	decision := policydomain.EvaluateCommandDecision(req, facts)
	result.DenyReasons = decision.DenyReasons
	result.Warnings = decision.Warnings
	result.Allowed = decision.Allowed
	return result
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
