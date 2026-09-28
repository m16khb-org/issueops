package issueopscycle

import model "issueops/internal/contract/issueops"

type AISlopCleanRefreshStore struct {
	Readiness   func(model.IssueOpsRecord) model.IssueOpsReadiness
	Now         func() string
	Head        func(model.IssueOpsRecord) string
	Fingerprint func(model.IssueOpsRecord) string
	TouchWrite  func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
}
