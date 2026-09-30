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
	lookup policyapp.PreparedBaseBranchLookup
}

func NewEvaluator(lookup policyapp.PreparedBaseBranchLookup) Evaluator {
	return Evaluator{lookup: lookup}
}

func (e Evaluator) Evaluate(req policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation {
	return e.service().Evaluate(req)
}

type CommandObserver struct{}

func (observer CommandObserver) Observe(req policycontract.CommandPolicyRequest) policyapp.Observation {
	root := absOrOriginal(req.WorkspaceRoot)
	cwd := absOrOriginal(req.CWD)
	canonicalRoot := canonicalPotentialPath(root)
	canonicalCWD := canonicalPotentialPath(cwd)
	argv := append([]string{}, req.Argv...)
	auditID := req.AuditLogID
	if auditID == "" {
		auditID = auditid.Generate(req.WorkspaceRoot, req.CWD, req.Argv)
	}
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	facts := policydomain.CommandFacts{
		RootDirectory:   isDirectory(root),
		CWDDirectory:    isDirectory(cwd),
		CWDWithinRoot:   sameOrWithin(canonicalRoot, canonicalCWD),
		PathOutsideRoot: commandReferencesOutsideWorkspace(canonicalRoot, canonicalCWD, argv),
	}
	return policyapp.Observation{
		Root: root, CWD: cwd, AuditLogID: auditID,
		GeneratedAt: generatedAt, Facts: facts,
	}
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
