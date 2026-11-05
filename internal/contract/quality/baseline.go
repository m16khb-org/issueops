package quality

// SNRBaselineSchemaVersion is the schema_version stored in every SNRBaselineRecord.
const SNRBaselineSchemaVersion = 1

// SNRBaselineRecord is the persisted code signal-to-noise baseline of one repository, compared against later runs by the trend gate.
type SNRBaselineRecord struct {
	SchemaVersion int     `json:"schema_version"`
	Repository    string  `json:"repository"`
	Ratio         float64 `json:"ratio"`
}
