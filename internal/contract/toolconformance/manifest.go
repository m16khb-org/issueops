// Package toolconformance defines the data contract for tool-call conformance: the fixtures, observations,
// classifications, per-episode and per-host reports and gate decisions used to check whether a model's
// tool-call arguments match an advertised input schema. It holds types and parsers only.
package toolconformance

// FixtureManifest is the versioned fixture file listing the conformance fixtures and the baseline cases
// expected to classify identically on every run.
type FixtureManifest struct {
	SchemaVersion int            `json:"schema_version"`
	Fixtures      []Fixture      `json:"fixtures"`
	BaselineCases []BaselineCase `json:"baseline_cases"`
}
