// Package issueopscycle declares the outbound ports of the cycle application layer: the readiness checks,
// git observations, clocks, and record stores that phase transitions, gates, and refreshes depend on. Adapters
// supply the implementations; the application layer only sees these function sets and interfaces.
package issueopscycle

import model "issueops/internal/contract/issueops"

// AISlopCleanRefreshStore bundles the observations and write that refreshing a cycle's ai-slop-clean phase needs:
// the phase readiness check, the clock, the current head and change fingerprint, and the record write.
type AISlopCleanRefreshStore struct {
	Readiness   func(model.Record) model.Readiness
	Now         func() string
	Head        func(model.Record) string
	Fingerprint func(model.Record) string
	TouchWrite  func(string, model.Record) (model.Record, error)
}
