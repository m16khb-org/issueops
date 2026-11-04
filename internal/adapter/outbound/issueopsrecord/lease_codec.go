package issueopsrecord

import (
	leasecontract "issueops/internal/contract/issueopslease"
	statecontract "issueops/internal/contract/state"
	leasedomain "issueops/internal/domain/issueopslease"
)

// DecodeLease parses persisted lease-vertical JSON for id and checks the lease
// domain's persisted-record rules. A record that fails either step is
// reported as an invalid-state error.
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

// EncodeLease validates record against the lease schema version and the lease
// domain's persisted-record rules and returns its JSON.
func EncodeLease(record leasecontract.Record) ([]byte, error) {
	if record.SchemaVersion != leasecontract.SchemaVersion {
		return nil, statecontract.Invalid("")
	}
	if err := leasedomain.ValidatePersistedRecord(record); err != nil {
		return nil, err
	}
	return leasecontract.Encode(record)
}
