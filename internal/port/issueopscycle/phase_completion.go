package issueopscycle

import model "issueops/internal/contract/issueops"

// PhaseCompletionReadiness holds the readiness checks used to decide whether a phase can complete:
// compatibility, ai-slop-clean, and pr readiness, plus the missing remote artifact items.
type PhaseCompletionReadiness struct {
	Compatibility         func(model.Record) model.Readiness
	AISlopClean           func(model.Record) model.Readiness
	PR                    func(model.Record) model.Readiness
	RemoteArtifactMissing func(model.Record) []string
}
