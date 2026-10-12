package failurecause

import (
	"strings"
	"testing"

	contract "issueops/internal/contract/failurecause"
)

func TestClassifyKeepsTypedEvidencePriority(t *testing.T) {
	result := Classify(true, []contract.Evidence{{Cause: contract.Model, Code: "model"}, {Cause: contract.Transport, Code: "network"}})
	if result.Cause != contract.Transport || result.Reason != "transport:network" {
		t.Fatalf("unexpected cause: %+v", result)
	}
}

func TestClassifyKeepsUnknownAndDropsUnrecognizedCauses(t *testing.T) {
	result := Classify(true, []contract.Evidence{{Cause: contract.Unknown, Code: "u"}, {Cause: contract.Cause("bogus"), Code: "b"}})
	if len(result.Evidence) != 1 || result.Evidence[0].Cause != contract.Unknown {
		t.Fatalf("typed evidence = %+v", result.Evidence)
	}
}

func TestClassifyTokenCharacterAndLengthBoundaries(t *testing.T) {
	result := Classify(true, []contract.Evidence{{Cause: contract.Model, Code: "az09", Source: "a`{/:@-"}})
	if got := result.Evidence[0].Code; got != "az09" {
		t.Fatalf("code = %q", got)
	}
	if got := result.Evidence[0].Source; got != "a_:_-" {
		t.Fatalf("source = %q", got)
	}
	long := strings.Repeat("a", 97)
	result = Classify(true, []contract.Evidence{{Cause: contract.Model, Code: long}})
	if got := result.Evidence[0].Code; got != long[:96] {
		t.Fatalf("code length = %d, want 96", len(got))
	}
}
