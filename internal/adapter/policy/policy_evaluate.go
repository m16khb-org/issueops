// Package policy is the outbound adapter of the command policy: it observes the file system, runs allowed read-only commands under a timeout in their own process group, and reads policy overrides. The decisions themselves come from the application and domain policy packages.
package policy

import (
	"os"
	"time"

	policyapp "issueops/internal/application/policy"
	policycontract "issueops/internal/contract/policy"
	policydomain "issueops/internal/domain/policy"

	"issueops/internal/domain/auditid"
)

// Evaluator runs command policy requests by wiring the file system observer, override loader, executor and clock into the policy application service.
type Evaluator struct {
	lookup policyapp.PreparedBaseBranchLookup
}

// NewEvaluator returns an Evaluator that resolves prepared base branches through lookup.
func NewEvaluator(lookup policyapp.PreparedBaseBranchLookup) Evaluator {
	return Evaluator{lookup: lookup}
}

// CommandObserver gathers the file system facts (directory checks, workspace containment, paths outside the root) a command policy decision needs.
type CommandObserver struct{}

// Observe resolves the workspace root and cwd to absolute paths and records whether they are directories, whether cwd is inside the root, and whether argv references paths outside it. It generates an audit log ID when the request has none.
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
