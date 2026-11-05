package issueopscycle

import model "issueops/internal/contract/issueops"

// PRPhaseGuard supplies what the pr phase guard needs: Read loads the cycle record and Gate judges its
// readiness for entering the pr phase.
type PRPhaseGuard struct {
	Read func(stateRoot, id string) (model.Record, error)
	Gate func(model.Record) model.Readiness
}
