package policy

import (
	"os"
	"time"

	policyapp "issueops/internal/application/policy"
	policycontract "issueops/internal/contract/policy"
	policydomain "issueops/internal/domain/policy"

	"issueops/internal/domain/auditid"
)

func EvaluateCommandPolicy(req policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation {
	return (policyapp.Service{Observer: commandObserver{}}).Evaluate(req)
}

type commandObserver struct{}

func (commandObserver) Observe(req policycontract.CommandPolicyRequest) policyapp.Observation {
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
	generatedAt := time.Now().UTC().Format(time.RFC3339)
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
	return policyapp.Observation{
		Root: root, CWD: cwd, Timeout: timeout, AuditLogID: auditID,
		GeneratedAt: generatedAt, Facts: facts,
	}
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
