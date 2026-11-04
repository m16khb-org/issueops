package issueopscycle

import model "issueops/internal/contract/issueops"

// PhaseTransitionObservations supplies the values a phase transition stamps on the record: the current time,
// the worktree head, and the change fingerprint.
type PhaseTransitionObservations struct {
	Now         func() string
	Head        func(model.Record) string
	Fingerprint func(model.Record) string
}
