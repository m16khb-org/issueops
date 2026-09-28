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
	return (Evaluator{}).Evaluate(req)
}

type Evaluator struct {
	lookup PreparedBaseBranchLookup
}

func NewEvaluator(lookup PreparedBaseBranchLookup) Evaluator { return Evaluator{lookup: lookup} }

func (e Evaluator) Evaluate(req policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation {
	return (policyapp.Service{Observer: commandObserver{lookup: e.lookup}}).Evaluate(req)
}

type commandObserver struct{ lookup PreparedBaseBranchLookup }

func (observer commandObserver) Observe(req policycontract.CommandPolicyRequest) policyapp.Observation {
	root := absOrOriginal(req.WorkspaceRoot)
	cwd := absOrOriginal(req.CWD)
	catalog, catalogWarnings := policyCatalogForWorkspace(root)
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
		Warnings:        catalogWarnings,
		PathOutsideRoot: commandReferencesOutsideWorkspace(canonicalRoot, canonicalCWD, argv),
	}
	if len(argv) > 0 {
		classification := catalog.Classify(argv)
		facts.ShellCommand = classification.ShellCommand
		facts.UsesNetwork = classification.UsesNetwork
		facts.Writes = classification.Writes
		facts.ReadOnlyAllowed = classification.ReadOnlyAllowed
		facts.PRTargetDeny, facts.PRTargetExpected = pullRequestTargetDeny(root, cwd, argv, observer.lookup)
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
