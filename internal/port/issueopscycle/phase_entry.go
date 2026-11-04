package issueopscycle

import model "issueops/internal/contract/issueops"

type PhaseEntryReadiness struct {
	Problem               func(model.Record) model.Readiness
	Grill                 func(model.Record) model.Readiness
	Plan                  func(model.Record) model.Readiness
	Compatibility         func(model.Record) model.Readiness
	Implement             func(model.Record) model.Readiness
	AISlopClean           func(model.Record) model.Readiness
	StrictPR              func(model.Record) model.Readiness
	RemoteArtifactMissing func(model.Record) []string
}
