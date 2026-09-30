package failurecause

import (
	"testing"

	contract "issueops/internal/contract/failurecause"
)

func TestClassifyKeepsTypedEvidencePriority(t *testing.T) {
	result := Classify(true, []contract.Evidence{{Cause: contract.Model, Code: "model"}, {Cause: contract.Transport, Code: "network"}})
	if result.Cause != contract.Transport || result.Reason != "transport:network" {
		t.Fatalf("unexpected cause: %+v", result)
	}
}
