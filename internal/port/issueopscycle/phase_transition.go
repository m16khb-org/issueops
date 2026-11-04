package issueopscycle

import model "issueops/internal/contract/issueops"

type PhaseTransitionObservations struct {
	Now         func() string
	Head        func(model.Record) string
	Fingerprint func(model.Record) string
}
