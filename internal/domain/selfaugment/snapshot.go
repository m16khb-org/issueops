package selfaugment

import (
	"fmt"

	contract "issueops/internal/contract/selfaugment"
)

// ValidateSummarySnapshot checks that the snapshot stored under key is a self-verification summary
// of supported schema version 1, and names key in the error otherwise.
func ValidateSummarySnapshot(key string, snapshot contract.StateSnapshot) error {
	if snapshot.Kind != SelfVerificationSummaryKind {
		return fmt.Errorf("state key %q contains kind %q, want %s", key, snapshot.Kind, SelfVerificationSummaryKind)
	}
	if snapshot.SchemaVersion != 1 {
		return fmt.Errorf("state key %q has unsupported self-verification summary schema %d", key, snapshot.SchemaVersion)
	}
	return nil
}

// IsSelfVerificationSummaryKind reports whether kind is the self-verification summary snapshot
// kind.
func IsSelfVerificationSummaryKind(kind string) bool { return kind == SelfVerificationSummaryKind }
