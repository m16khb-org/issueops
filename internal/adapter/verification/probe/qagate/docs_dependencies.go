package qagate

import (
	selfverify "issueops/internal/contract/selfverify"
)

type Validator struct {
	ListDocs   func(string) []string
	ListSkills func(string) ([]string, error)
}

func (v Validator) Validate(root string) selfverify.StepResult {
	return validateQAGateWithDeps(root, docsValidationDeps{listDocs: v.ListDocs, listSkills: v.ListSkills})
}

func (v Validator) RedactionAudit(root string) selfverify.StepResult {
	return validateRedactionAuditWithDeps(root, docsValidationDeps{listDocs: v.ListDocs, listSkills: v.ListSkills})
}
