package state

import statecontract "issueops/internal/contract/state"

// ValidateRecord checks a stored record against the key it was read under. The
// schema version must be current, the key must match, and Bytes must equal the
// content length. Any violation returns the record_invariant error.
func ValidateRecord(expectedKey string, record statecontract.RecordEnvelope) error {
	if record.SchemaVersion != statecontract.SchemaVersion || record.Key != expectedKey || record.Bytes != len([]byte(record.Content)) {
		return statecontract.Invalid("record_invariant")
	}
	return nil
}
