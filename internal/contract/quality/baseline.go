package quality

const SNRBaselineSchemaVersion = 1

type SNRBaselineRecord struct {
	SchemaVersion int     `json:"schema_version"`
	Repository    string  `json:"repository"`
	Ratio         float64 `json:"ratio"`
}
