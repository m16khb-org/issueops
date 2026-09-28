package issueopspreparation

import (
	"strings"
	"testing"
)

func TestValidateIntentStateRequiresRawCASBeforeAuthority(t *testing.T) {
	if err := ValidateIntentState(IntentState{}); err == nil || !strings.Contains(err.Error(), "raw CAS evidence is required") {
		t.Fatalf("validation error=%v", err)
	}
}
