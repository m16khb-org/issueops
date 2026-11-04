package state

import (
	statecontract "issueops/internal/contract/state"
	statedomain "issueops/internal/domain/state"
	"issueops/internal/domain/statepath"
)

// DoctorRow is one raw state record as read from storage: its key, the file
// path it came from, and the undecoded bytes. Doctor validates it without
// touching the store.
type DoctorRow struct {
	Key  string
	Path string
	Data []byte
}

// Doctor decodes each row, flags invalid keys and records (undecodable JSON,
// bad schema, unparsable UpdatedAt), and merges them with the directory
// entries into a DoctorResult. It is read-only and never returns an error;
// problems are reported inside the result.
func Doctor(dir string, entries []statedomain.DoctorEntry, rows []DoctorRow) statecontract.DoctorResult {
	observations := make([]statedomain.DoctorRecord, 0, len(rows))
	for _, row := range rows {
		observation := statedomain.DoctorRecord{Key: row.Key, Path: row.Path}
		if _, err := statepath.NormalizeKey(row.Key); err != nil {
			observation.InvalidKey = err.Error()
		} else {
			record, err := decodeRecord(row.Key, row.Data)
			if err == nil {
				_, err = statecontract.ParseTime(record.UpdatedAt)
			}
			observation.Record = record
			observation.InvalidRecord = err != nil
		}
		observations = append(observations, observation)
	}
	return statedomain.InspectDoctor(dir, entries, observations)
}
