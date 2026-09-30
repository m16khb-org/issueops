package toolconformance

type FixtureManifest struct {
	SchemaVersion int            `json:"schema_version"`
	Fixtures      []Fixture      `json:"fixtures"`
	BaselineCases []BaselineCase `json:"baseline_cases"`
}
