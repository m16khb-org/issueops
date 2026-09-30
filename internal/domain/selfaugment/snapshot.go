package selfaugment

import (
	"fmt"

	contract "issueops/internal/contract/selfaugment"
)

func ValidateSummarySnapshot(key string, snapshot contract.SelfAugmentStateSnapshot) error {
	if snapshot.Kind != SelfVerificationSummaryKind {
		return fmt.Errorf("state key %q contains kind %q, want %s", key, snapshot.Kind, SelfVerificationSummaryKind)
	}
	if snapshot.SchemaVersion != 1 {
		return fmt.Errorf("state key %q has unsupported self-verification summary schema %d", key, snapshot.SchemaVersion)
	}
	return nil
}

func IsSelfVerificationSummaryKind(kind string) bool { return kind == SelfVerificationSummaryKind }
