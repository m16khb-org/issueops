package issueopsrecord

import (
	leasecontract "issueops/internal/contract/issueopslease"
	statecontract "issueops/internal/contract/state"
	leasedomain "issueops/internal/domain/issueopslease"
)

func DecodeLease(id string, data []byte) (leasecontract.Record, error) {
	record, err := leasecontract.Decode(id, data)
	if err != nil {
		return leasecontract.Record{}, err
	}
	if err := leasedomain.ValidatePersistedRecord(record); err != nil {
		return leasecontract.Record{}, statecontract.Invalid("")
	}
	return record, nil
}

func EncodeLease(record leasecontract.Record) ([]byte, error) {
	if record.SchemaVersion != leasecontract.SchemaVersion {
		return nil, statecontract.Invalid("")
	}
	if err := leasedomain.ValidatePersistedRecord(record); err != nil {
		return nil, err
	}
	return leasecontract.Encode(record)
}
