package state

// SchemaVersion is the schema_version written to every RecordEnvelope.
const SchemaVersion = 1

// RecordEnvelope is the stored form of one state checkpoint: its key, content, update time (RFC 3339) and content size in bytes.
type RecordEnvelope struct {
	SchemaVersion int    `json:"schema_version"`
	Key           string `json:"key"`
	Content       string `json:"content"`
	UpdatedAt     string `json:"updated_at"`
	Bytes         int    `json:"bytes"`
}
