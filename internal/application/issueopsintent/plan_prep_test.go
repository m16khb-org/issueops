package issueopsintent

import (
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	intentport "issueops/internal/port/issueopsintent"
)

func TestRecordPlanPrepValidatesAllItemsBeforeRead(t *testing.T) {
	request := model.PlanPrepRequest{
		PriorDecisions: model.PlanPrepItemRequest{Evidence: []string{"decision"}},
		RelatedIssues:  model.PlanPrepItemRequest{Evidence: []string{"issue"}},
		WebResearch:    model.PlanPrepItemRequest{WaiveReason: "internal change"},
	}
	store := intentport.Store{Read: func(_, _ string) (model.Record, error) {
		t.Fatal("invalid plan preparation must fail before read")
		return model.Record{}, nil
	}}
	_, err := RecordPlanPrep(store, "state", "io-1", request)
	if err == nil || !strings.Contains(err.Error(), "codebase_survey") {
		t.Fatalf("err=%v", err)
	}
}

func TestRecordPlanPrepWritesPreparedItemsWithInjectedTime(t *testing.T) {
	var events []string
	store := intentport.Store{
		Read: func(_, _ string) (model.Record, error) {
			events = append(events, "read")
			return model.Record{ID: "io-1"}, nil
		},
		Now: func() string { events = append(events, "clock"); return "2026-09-28T00:00:00Z" },
		TouchWrite: func(_ string, record model.Record) (model.Record, error) {
			events = append(events, "write")
			return record, nil
		},
	}
	record, err := RecordPlanPrep(store, "state", "io-1", model.PlanPrepRequest{
		PriorDecisions: model.PlanPrepItemRequest{Evidence: []string{"decision"}},
		RelatedIssues:  model.PlanPrepItemRequest{Evidence: []string{"issue"}},
		WebResearch:    model.PlanPrepItemRequest{WaiveReason: "internal change"},
		CodebaseSurvey: model.PlanPrepItemRequest{Evidence: []string{"source survey"}},
	})
	if err != nil || !reflect.DeepEqual(events, []string{"read", "clock", "write"}) || record.PlanPrep == nil ||
		record.PlanPrep.WebResearch.Status != "waived" || record.PlanPrep.RecordedAt != "2026-09-28T00:00:00Z" {
		t.Fatalf("record=%+v err=%v events=%v", record, err, events)
	}
}
