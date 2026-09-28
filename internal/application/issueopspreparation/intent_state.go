package issueopspreparation

import (
	"fmt"

	preparationdomain "issueops/internal/domain/issueopspreparation"
)

func ValidateIntentState(state IntentState) error {
	if len(state.Snapshot.RecordRaw) == 0 || len(state.IntentRaw) == 0 {
		return fmt.Errorf("Orca intent raw CAS evidence is required")
	}
	return preparationdomain.ValidateIntentRecord(state.Snapshot.Record, state.Intent)
}
