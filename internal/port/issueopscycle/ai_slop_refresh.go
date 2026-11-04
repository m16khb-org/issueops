package issueopscycle

import model "issueops/internal/contract/issueops"

type AISlopCleanRefreshStore struct {
	Readiness   func(model.Record) model.Readiness
	Now         func() string
	Head        func(model.Record) string
	Fingerprint func(model.Record) string
	TouchWrite  func(string, model.Record) (model.Record, error)
}
