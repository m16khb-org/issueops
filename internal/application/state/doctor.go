package state

import (
	statecontract "issueops/internal/contract/state"
	statedomain "issueops/internal/domain/state"
	"issueops/internal/domain/statepath"
)

type DoctorEntry = statedomain.DoctorEntry

type DoctorRow struct {
	Key  string
	Path string
	Data []byte
}

func Doctor(dir string, entries []DoctorEntry, rows []DoctorRow) statecontract.StateDoctorResult {
	observations := make([]statedomain.DoctorRecord, 0, len(rows))
	for _, row := range rows {
		observation := statedomain.DoctorRecord{Key: row.Key, Path: row.Path}
		if _, err := statepath.NormalizeKey(row.Key); err != nil {
			observation.InvalidKey = err.Error()
		} else {
			record, err := DecodeRecord(row.Key, row.Data)
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
