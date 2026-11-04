// Package qagate holds the self-verify probes behind the QA gate: documentation and
// skill validation, mermaid diagram linting, and the redaction audit.
package qagate

import (
	selfverify "issueops/internal/contract/selfverify"
)

// Validator runs the QA gate probes with the docs and skills listings injected so
// the adapter needs no direct knowledge of how they are produced.
type Validator struct {
	ListDocs   func(string) []string
	ListSkills func(string) ([]string, error)
}

// Validate runs the QA gate checks over the repository at root.
func (v Validator) Validate(root string) selfverify.StepResult {
	return validateQAGateWithDeps(root, docsValidationDeps{listDocs: v.ListDocs, listSkills: v.ListSkills})
}

// RedactionAudit scans the repository's docs and skills at root for unredacted
// secret patterns.
func (v Validator) RedactionAudit(root string) selfverify.StepResult {
	return validateRedactionAuditWithDeps(root, docsValidationDeps{listDocs: v.ListDocs, listSkills: v.ListSkills})
}
