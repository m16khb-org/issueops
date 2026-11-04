package issueops

import "testing"

func TestValidateRecordAcceptsEveryCurrentPhase(t *testing.T) {
	for _, phase := range Phases {
		t.Run(string(phase), func(t *testing.T) {
			record := Record{
				SchemaVersion: SchemaVersion,
				ID:            "io-valid",
				Phase:         phase,
				PhaseLedger: PhaseLedger{
					phase: {Phase: phase},
				},
			}

			if err := ValidateRecord(record); err != nil {
				t.Fatalf("ValidateRecord(%q): %v", phase, err)
			}
		})
	}
}

func TestValidateRecordAcceptsCompletePlanPreparation(t *testing.T) {
	evidence := PlanPrepItem{Status: "evidence", Evidence: []string{"source"}}
	waived := PlanPrepItem{Status: "waived", WaiveReason: "not needed"}
	record := Record{
		SchemaVersion: SchemaVersion,
		ID:            "io-planprep",
		Phase:         PhasePlan,
		PlanPrep: &PlanPrep{
			PriorDecisions: evidence,
			RelatedIssues:  waived,
			WebResearch:    evidence,
			CodebaseSurvey: waived,
		},
	}

	if err := ValidateRecord(record); err != nil {
		t.Fatalf("ValidateRecord: %v", err)
	}
}

func TestValidateRecordLeavesPlanPreparationPolicyToDomain(t *testing.T) {
	tests := []PlanPrepItem{
		{Status: "evidence", Evidence: []string{"source"}, WaiveReason: "also waived"},
		{Status: "waived"},
		{Status: "waived", Evidence: []string{"source"}, WaiveReason: "not needed"},
	}
	for index, item := range tests {
		record := Record{
			SchemaVersion: SchemaVersion,
			ID:            "io-invalid-planprep",
			Phase:         PhasePlan,
			PlanPrep: &PlanPrep{
				PriorDecisions: item,
			},
		}

		if err := ValidateRecord(record); err != nil {
			t.Fatalf("case %d failed shape validation: %v", index, err)
		}
	}
}
