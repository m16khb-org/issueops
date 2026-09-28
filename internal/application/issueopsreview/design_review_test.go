package issueopsreview

import (
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func TestRecordDesignReviewPreservesValidationReadinessAndWriteOrder(t *testing.T) {
	request := model.IssueOpsDesignReviewRequest{
		ProblemSummary: "quality signal is low", ProposedDesign: "add focused tests",
		RefactorPlan: "keep production unchanged", Alternatives: []string{"raise threshold"},
		Risks: []string{"brittle tests"}, Verification: []string{"design review checked risks"}, Approved: true,
	}
	for _, tc := range []struct {
		name    string
		request model.IssueOpsDesignReviewRequest
		ready   model.IssueOpsReadiness
		want    string
		events  []string
	}{
		{name: "invalid request before read", request: model.IssueOpsDesignReviewRequest{}, want: "problem_summary is required"},
		{name: "intent missing", request: request, ready: model.IssueOpsReadiness{Missing: []string{"intent"}}, want: "cannot record design review before intent contract", events: []string{"read", "readiness"}},
		{name: "plan preparation alone is allowed", request: request, ready: model.IssueOpsReadiness{Missing: []string{"plan_prep_evidence"}}, events: []string{"read", "readiness", "clock", "write"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			store := reviewport.DesignReviewStore{
				Read: func(_, _ string) (model.IssueOpsRecord, error) {
					events = append(events, "read")
					return model.IssueOpsRecord{ID: "io-1"}, nil
				},
				PlanReadiness: func(model.IssueOpsRecord) model.IssueOpsReadiness {
					events = append(events, "readiness")
					return tc.ready
				},
				Now: func() string { events = append(events, "clock"); return "2026-09-28T00:00:00Z" },
				TouchWrite: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
					events = append(events, "write")
					if record.DesignReview == nil || record.DesignReview.ReviewedAt != "2026-09-28T00:00:00Z" {
						t.Fatalf("design review = %+v", record.DesignReview)
					}
					return record, nil
				},
			}
			_, err := RecordDesignReview(store, "state", "io-1", tc.request)
			if (tc.want == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), tc.want)) || !reflect.DeepEqual(events, tc.events) {
				t.Fatalf("err=%v events=%v want=%v", err, events, tc.events)
			}
		})
	}
}
