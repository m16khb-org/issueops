package issueopscycle

import model "issueops/internal/contract/issueops"

type PRPhaseGuard struct {
	Read func(stateRoot, id string) (model.Record, error)
	Gate func(model.Record) model.Readiness
}
