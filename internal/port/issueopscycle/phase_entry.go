package issueopscycle

import model "issueops/internal/contract/issueops"

// PhaseEntryReadiness holds one readiness check per phase that must pass before the cycle may enter that
// phase, plus the missing remote artifact items. Each check reports what is still missing for the record.
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
