package issueopsstatus

import (
	"reflect"
	"slices"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestProjectorDerivesDeterministicLedger(t *testing.T) {
	projector := NewProjector(func(
		_ issueopscontract.Record,
		phase issueopscontract.Phase,
	) issueopscontract.Readiness {
		if phase == issueopscontract.PhaseProblem {
			return issueopscontract.Readiness{Ready: true}
		}
		return issueopscontract.Readiness{Missing: []string{"missing_" + string(phase)}}
	})
	record := issueopscontract.Record{Phase: issueopscontract.PhaseGrill}

	left := projector.Derive(record)
	right := projector.Derive(record)
	if !reflect.DeepEqual(left, right) {
		t.Fatalf("derived ledger is not deterministic:\nleft=%#v\nright=%#v", left, right)
	}
	if problem := left[issueopscontract.PhaseProblem]; len(problem.Artifacts) == 0 || len(problem.Missing) != 0 {
		t.Fatalf("ready phase projection is incomplete: %#v", problem)
	}
	if grill := left[issueopscontract.PhaseGrill]; len(grill.Missing) == 0 {
		t.Fatalf("incomplete phase must expose missing keys: %#v", grill)
	}
}

func TestProjectorBackfillsWithoutReplacingPersistedEntries(t *testing.T) {
	var asked []issueopscontract.Phase
	projector := NewProjector(func(
		_ issueopscontract.Record,
		phase issueopscontract.Phase,
	) issueopscontract.Readiness {
		asked = append(asked, phase)
		return issueopscontract.Readiness{Ready: true}
	})
	record := issueopscontract.Record{
		Phase: issueopscontract.PhasePlan,
		PhaseLedger: issueopscontract.PhaseLedger{
			issueopscontract.PhaseProblem: {
				Phase:       issueopscontract.PhaseProblem,
				CompletedAt: "2026-06-29T00:01:00Z",
			},
		},
	}

	projected := projector.Project(record)
	if len(projected.PhaseLedger) != len(issueopscontract.Phases) {
		t.Fatalf("partial ledger was not backfilled: %#v", projected.PhaseLedger)
	}
	if got := projected.PhaseLedger[issueopscontract.PhaseProblem].CompletedAt; got != "2026-06-29T00:01:00Z" {
		t.Fatalf("persisted entry was replaced: %q", got)
	}
	// Completion observes git; a recorded phase must not pay for it.
	if slices.Contains(asked, issueopscontract.PhaseProblem) || len(asked) != len(issueopscontract.Phases)-1 {
		t.Fatalf("completion asked for %v, want every phase except the recorded problem", asked)
	}
	if _, mutated := record.PhaseLedger[issueopscontract.PhasePlan]; mutated {
		t.Fatal("projection mutated the caller's stored ledger")
	}
}

func TestArtifactKeysIndexChildrenOnlyAtPR(t *testing.T) {
	if !slices.Contains(ArtifactKeys(issueopscontract.PhasePR), "children_complete") {
		t.Fatal("PR artifact keys must include children_complete")
	}
	if slices.Contains(ArtifactKeys(issueopscontract.PhaseImplement), "children_complete") {
		t.Fatal("implement artifact keys must not include children_complete")
	}
}
