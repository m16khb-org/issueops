package qagate

type Validator struct {
	ListDocs   func(string) []string
	ListSkills func(string) ([]string, error)
}

func (v Validator) Validate(root string) StepResult {
	return validateQAGateWithDeps(root, docsValidationDeps{listDocs: v.ListDocs, listSkills: v.ListSkills})
}
func (v Validator) MermaidDocs(root string) []string {
	return validateMermaidDocsWithDeps(root, docsValidationDeps{listDocs: v.ListDocs, listSkills: v.ListSkills})
}

func (v Validator) RedactionAudit(root string) StepResult {
	return validateRedactionAuditWithDeps(root, docsValidationDeps{listDocs: v.ListDocs, listSkills: v.ListSkills})
}
