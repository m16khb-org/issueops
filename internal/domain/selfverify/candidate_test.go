package selfverify

import (
	"testing"

	contract "issueops/internal/contract/selfverify"
)

func TestSelectedCandidatePicksHighestScoreThenLowestPriority(t *testing.T) {
	open := contract.CandidateStatusOpen
	cands := []contract.SelfVerificationCandidate{
		{ID: "sat", Status: contract.CandidateStatusSatisfied, Score: 99, Priority: 1},
		{ID: "a", Status: open, Score: 70, Priority: 1},
		{ID: "b", Status: open, Score: 80, Priority: 5},
		{ID: "c", Status: open, Score: 80, Priority: 2},
	}
	got := SelectedCandidate(cands)
	if got == nil || got.ID != "c" || SelectedCandidateID(got) != "c" {
		t.Fatalf("selected=%+v", got)
	}
	got.ID = "mutated"
	if cands[3].ID != "c" {
		t.Fatal("selection must be a copy")
	}
	if SelectedCandidate(cands[:1]) != nil || SelectedCandidateID(nil) != "none" {
		t.Fatal("no open candidate must yield nil / none")
	}
}

func TestCandidateIDsByStatusAndCatalogInvariants(t *testing.T) {
	catalog := CandidateCatalog()
	seen := map[string]bool{}
	for i, c := range catalog {
		if c.Priority != i+1 || seen[c.ID] || c.ID == "" {
			t.Fatalf("catalog entry %d invalid: %+v", i, c)
		}
		seen[c.ID] = true
	}
	satisfied := CandidateIDsByStatus(catalog, contract.CandidateStatusSatisfied)
	open := CandidateIDsByStatus(catalog, contract.CandidateStatusOpen)
	if len(satisfied)+len(open) != len(catalog) || open == nil {
		t.Fatalf("satisfied=%d open=%v", len(satisfied), open)
	}
}

func TestCoverageForLabelsWithNoLabelsReportsEveryClaimMissing(t *testing.T) {
	coverage, gaps := CoverageForLabels(nil)
	if len(gaps) == 0 {
		t.Fatal("no labels must produce gaps")
	}
	for _, item := range coverage {
		if item.Covered || len(item.MissingLabels) != len(item.EvidenceLabels) {
			t.Fatalf("item=%+v", item)
		}
	}
}

func TestCandidateIDsByStatusFiltersExactStatusInOrder(t *testing.T) {
	cands := []contract.SelfVerificationCandidate{
		{ID: "a", Status: contract.CandidateStatusOpen},
		{ID: "b", Status: contract.CandidateStatusSatisfied},
		{ID: "c", Status: contract.CandidateStatusOpen},
	}
	got := CandidateIDsByStatus(cands, contract.CandidateStatusOpen)
	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Fatalf("open ids=%v", got)
	}
}

func TestSelectedCandidateKeepsFirstOnTies(t *testing.T) {
	open := contract.CandidateStatusOpen
	tests := []struct {
		name  string
		cands []contract.SelfVerificationCandidate
	}{
		{"equal score, later has worse priority", []contract.SelfVerificationCandidate{
			{ID: "first", Status: open, Score: 80, Priority: 1},
			{ID: "second", Status: open, Score: 80, Priority: 2},
		}},
		{"equal score and priority", []contract.SelfVerificationCandidate{
			{ID: "first", Status: open, Score: 80, Priority: 3},
			{ID: "second", Status: open, Score: 80, Priority: 3},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SelectedCandidate(tt.cands); got == nil || got.ID != "first" {
				t.Fatalf("selected=%+v", got)
			}
		})
	}
}
