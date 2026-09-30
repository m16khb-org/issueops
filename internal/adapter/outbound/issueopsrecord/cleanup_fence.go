package issueopsrecord

import (
	leasecontract "issueops/internal/contract/issueopslease"
	issueopsdomain "issueops/internal/domain/issueops"
)

// RequireMutableLeaseSnapshot checks both the supplied projection and the exact
// bytes used by CAS. A projection may omit an attempt already present in those
// bytes; CAS alone would then accept an overwrite of the armed record.
func RequireMutableLeaseSnapshot(record leasecontract.Record, raw []byte) error {
	if err := issueopsdomain.RequireNoCleanupAttempt(record.CleanupAttempt); err != nil {
		return err
	}
	persisted, err := DecodeLease(record.ID, raw)
	if err != nil {
		return err
	}
	return issueopsdomain.RequireNoCleanupAttempt(persisted.CleanupAttempt)
}
