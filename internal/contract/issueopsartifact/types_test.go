package issueopsartifact

import (
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestSealedErrorContract(t *testing.T) {
	err := &SealedError{ID: "io-42"}

	if got := err.Error(); got != "artifacts are sealed after execution prepare" {
		t.Fatalf("SealedError message = %q", got)
	}
	fields := err.IssueOpsErrorFields()
	if fields["code"] != "artifact_stage_sealed" {
		t.Fatalf("code = %v, want artifact_stage_sealed", fields["code"])
	}
	next, _ := fields["next_command"].(string)
	if !strings.Contains(next, "io-42") {
		t.Fatalf("next_command %q must reference cycle id", next)
	}
}

func TestAliasesShareUnderlyingIssueOpsContract(t *testing.T) {
	var record issueopscontract.IssueOpsRecord = Record{OK: true, ID: "io-1"}
	if !record.OK || record.ID != "io-1" {
		t.Fatalf("Record alias mismatch: %+v", record)
	}

	staged := Staged{"plan": "# plan"}
	if staged["plan"] != "# plan" {
		t.Fatalf("Staged alias mismatch: %+v", staged)
	}

	if PhaseDone != issueopscontract.IssueOpsPhaseDone {
		t.Fatal("PhaseDone alias drifted from source contract")
	}
	if MaxBytes <= 0 {
		t.Fatalf("MaxBytes = %d, want positive limit", MaxBytes)
	}
}
