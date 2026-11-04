package issueopscycle

import model "issueops/internal/contract/issueops"

type PhaseCompletionReadiness struct {
	Compatibility         func(model.Record) model.Readiness
	AISlopClean           func(model.Record) model.Readiness
	PR                    func(model.Record) model.Readiness
	RemoteArtifactMissing func(model.Record) []string
}
