package issueopscycle

import model "issueops/internal/contract/issueops"

type PhaseTransitionObservations struct {
	Now         func() string
	Head        func(model.IssueOpsRecord) string
	Fingerprint func(model.IssueOpsRecord) string
}
