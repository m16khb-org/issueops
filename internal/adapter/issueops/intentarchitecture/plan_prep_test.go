package intentarchitecture

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func planPrepMemStore() (Store, *model.Record) {
	rec := &model.Record{ID: "io-1"}
	store := Store{
		Read: func(_, _ string) (model.Record, error) { return *rec, nil },
		TouchWrite: func(_ string, r model.Record) (model.Record, error) {
			*rec = r
			return r, nil
		},
		PlanReadiness: func(r model.Record) model.Readiness {
			return model.Readiness{Ready: true}
		},
	}
	return store, rec
}

func TestRecordPlanPrepStoresEvidenceAndWaive(t *testing.T) {
	store, rec := planPrepMemStore()
	_, err := RecordPlanPrep(store, "state", "io-1", model.PlanPrepRequest{
		PriorDecisions: model.PlanPrepItemRequest{Evidence: []string{".issueops/ADR.md"}},
		RelatedIssues:  model.PlanPrepItemRequest{Evidence: []string{"remote score: selected=#1(0.9)"}},
		WebResearch:    model.PlanPrepItemRequest{WaiveReason: "내부 전용 변경"},
		CodebaseSurvey: model.PlanPrepItemRequest{Evidence: []string{"rg gate: issueops_readiness.go, plan_prep.go 관련 심볼 전수 확인"}},
	})
	if err != nil {
		t.Fatalf("RecordPlanPrep error: %v", err)
	}
	if rec.PlanPrep == nil || rec.PlanPrep.PriorDecisions.Status != "evidence" || rec.PlanPrep.WebResearch.Status != "waived" {
		t.Fatalf("unexpected plan_prep: %#v", rec.PlanPrep)
	}
	if rec.PlanPrep.CodebaseSurvey.Status != "evidence" {
		t.Fatalf("codebase_survey not persisted: %#v", rec.PlanPrep)
	}
}

func TestRecordPlanPrepRejectsMissingCodebaseSurvey(t *testing.T) {
	store, _ := planPrepMemStore()
	_, err := RecordPlanPrep(store, "state", "io-1", model.PlanPrepRequest{
		PriorDecisions: model.PlanPrepItemRequest{Evidence: []string{"a"}},
		RelatedIssues:  model.PlanPrepItemRequest{Evidence: []string{"b"}},
		WebResearch:    model.PlanPrepItemRequest{WaiveReason: "c"},
	})
	if err == nil {
		t.Fatal("codebase_survey with neither evidence nor waive must error")
	}
}

func TestRecordPlanPrepRejectsBothEvidenceAndWaive(t *testing.T) {
	store, _ := planPrepMemStore()
	_, err := RecordPlanPrep(store, "state", "io-1", model.PlanPrepRequest{
		PriorDecisions: model.PlanPrepItemRequest{Evidence: []string{"a"}, WaiveReason: "b"},
		RelatedIssues:  model.PlanPrepItemRequest{Evidence: []string{"a"}},
		WebResearch:    model.PlanPrepItemRequest{WaiveReason: "c"},
	})
	if err == nil {
		t.Fatal("evidence + waive on one item must error")
	}
}

func TestRecordPlanPrepRejectsEmptyItem(t *testing.T) {
	store, _ := planPrepMemStore()
	_, err := RecordPlanPrep(store, "state", "io-1", model.PlanPrepRequest{
		PriorDecisions: model.PlanPrepItemRequest{},
		RelatedIssues:  model.PlanPrepItemRequest{Evidence: []string{"a"}},
		WebResearch:    model.PlanPrepItemRequest{WaiveReason: "c"},
	})
	if err == nil {
		t.Fatal("item with neither evidence nor waive must error")
	}
}

func TestRecordIntentPersistsIntentClass(t *testing.T) {
	store, rec := planPrepMemStore()
	_, err := RecordIntent(store, "state", "io-1", model.IntentRecordRequest{
		RawRequest:        "raw ask",
		InterpretedIntent: "reframed agent interpretation differs",
		SuccessCriteria:   []string{"works"},
		IntentClass:       "architecture",
	})
	if err != nil {
		t.Fatalf("RecordIntent error: %v", err)
	}
	if rec.Intent == nil || rec.Intent.IntentClass != "architecture" {
		t.Fatalf("intent_class not persisted: %#v", rec.Intent)
	}
}

func TestRecordIntentRejectsUnknownIntentClass(t *testing.T) {
	store, _ := planPrepMemStore()
	_, err := RecordIntent(store, "state", "io-1", model.IntentRecordRequest{
		RawRequest:        "raw ask",
		InterpretedIntent: "reframed agent interpretation differs",
		SuccessCriteria:   []string{"works"},
		IntentClass:       "bogus",
	})
	if err == nil {
		t.Fatal("unknown intent_class must error")
	}
}
