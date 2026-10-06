package qualitycatalog

import (
	contract "issueops/internal/contract/qualitycatalog"
	"testing"
)

func TestApplyPlanStatusProjectsOnlyMatchingStatusAndScore(t *testing.T) {
	original := []contract.Candidate{{ID: "matched", Title: "Keep title", Status: "open", Score: 90}, {ID: "unmatched", Status: "open", Score: 50}}
	source := []contract.Candidate{{ID: original[0].ID, Status: "already_satisfied", Score: 12}, {ID: "outside-catalog", Status: "open", Score: 99}}
	got := ApplyPlanStatus(original, source)
	if len(got) != len(original) || got[0].Status != "already_satisfied" || got[0].Score != 12 || got[0].Title != original[0].Title {
		t.Fatalf("projection changed catalog identity: %+v", got)
	}
	if original[0].Status != "open" || got[1].Status != original[1].Status || got[1].Score != original[1].Score {
		t.Fatal("projection mutated catalog")
	}
}
