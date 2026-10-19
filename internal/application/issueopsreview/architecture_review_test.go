package issueopsreview

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func TestRecordArchitectureReviewPreservesValidationReadinessAndWriteOrder(t *testing.T) {
	request := reviewcontract.ArchitectureReviewRequest{
		ProblemSummary: "quality signal is low", ProposedArchitecture: "add focused tests",
		RefactorPlan: "keep production unchanged", Alternatives: []string{"raise threshold"},
		Risks: []string{"brittle tests"}, Verification: []string{"architecture review checked risks"}, Approved: true,
		ReviewerContext: "subagent",
	}
	for _, tc := range []struct {
		name    string
		request reviewcontract.ArchitectureReviewRequest
		ready   model.IssueOpsReadiness
		want    string
		events  []string
	}{
		{name: "invalid request before read", request: reviewcontract.ArchitectureReviewRequest{}, want: "problem_summary is required"},
		{name: "intent missing", request: request, ready: model.IssueOpsReadiness{Missing: []string{"intent"}}, want: "cannot record architecture review before intent contract", events: []string{"read", "readiness"}},
		{name: "plan preparation alone is allowed", request: request, ready: model.IssueOpsReadiness{Missing: []string{"plan_prep_evidence"}}, events: []string{"read", "readiness", "clock", "write"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			store := reviewport.ArchitectureReviewStore{
				Read: func(_, _ string) (model.IssueOpsRecord, error) {
					events = append(events, "read")
					return model.IssueOpsRecord{ID: "io-1"}, nil
				},
				PlanReadiness: func(model.IssueOpsRecord) model.IssueOpsReadiness {
					events = append(events, "readiness")
					return tc.ready
				},
				Now:      func() string { events = append(events, "clock"); return "2026-09-28T00:00:00Z" },
				Document: func(string, model.IssueOpsRecord) (string, error) { return validArchitectureDocument(), nil },
				TouchWrite: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
					events = append(events, "write")
					if record.ArchitectureReview == nil || record.ArchitectureReview.ReviewedAt != "2026-09-28T00:00:00Z" {
						t.Fatalf("architecture review = %+v", record.ArchitectureReview)
					}
					return record, nil
				},
			}
			_, err := RecordArchitectureReview(store, "state", "io-1", tc.request)
			if (tc.want == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), tc.want)) || !reflect.DeepEqual(events, tc.events) {
				t.Fatalf("err=%v events=%v want=%v", err, events, tc.events)
			}
		})
	}
}

func validArchitectureDocument() string {
	return strings.Join(issueopsdomain.RequiredArchitectureSections, "\n내용\n") + "\n내용\n"
}

func approvedArchitectureRequest() reviewcontract.ArchitectureReviewRequest {
	return reviewcontract.ArchitectureReviewRequest{
		ProblemSummary: "quality signal is low", ProposedArchitecture: "add focused tests",
		RefactorPlan: "keep production unchanged", Alternatives: []string{"raise threshold"},
		Risks: []string{"brittle tests"}, Verification: []string{"architecture review checked risks"}, Approved: true,
		ReviewerContext: "subagent",
	}
}

func TestRecordArchitectureReviewRejectsMissingSections(t *testing.T) {
	missingBody := strings.Replace(validArchitectureDocument(), "## 병목과 장애\n내용\n", "## 병목과 장애\n\n", 1)
	for _, tc := range []struct {
		name    string
		mutate  func(*reviewcontract.ArchitectureReviewRequest)
		doc     string
		want    string
		written bool
	}{
		{name: "no staged architecture", doc: "", want: "architecture review requires a staged architecture artifact"},
		{name: "empty section body names the heading", doc: missingBody, want: "architecture is missing required sections: ## 병목과 장애"},
		{name: "heading absent names every missing heading", doc: "## 요구사항과 규모\n내용\n", want: "## 데이터와 API 계약, ## 구조와 흐름, ## 병목과 장애, ## 절충점과 기각한 대안"},
		{name: "reviewer context is required", mutate: func(r *reviewcontract.ArchitectureReviewRequest) { r.ReviewerContext = "" }, doc: validArchitectureDocument(), want: "reviewer_context must be subagent or inline"},
		{name: "inherited context cannot be recorded", mutate: func(r *reviewcontract.ArchitectureReviewRequest) { r.ReviewerContext = "delegated-parent-review" }, doc: validArchitectureDocument(), want: "reviewer_context must be subagent or inline"},
		{name: "valid document is recorded", doc: validArchitectureDocument(), written: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := approvedArchitectureRequest()
			if tc.mutate != nil {
				tc.mutate(&request)
			}
			wrote := false
			store := reviewport.ArchitectureReviewStore{
				Read:          func(_, _ string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{ID: "io-1"}, nil },
				PlanReadiness: func(model.IssueOpsRecord) model.IssueOpsReadiness { return model.IssueOpsReadiness{Ready: true} },
				Now:           func() string { return "2026-09-28T00:00:00Z" },
				Document:      func(string, model.IssueOpsRecord) (string, error) { return tc.doc, nil },
				TouchWrite: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
					wrote = true
					return record, nil
				},
			}
			_, err := RecordArchitectureReview(store, "state", "io-1", request)
			if tc.written != wrote || (tc.want == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("err=%v wrote=%v, want error %q written=%v", err, wrote, tc.want, tc.written)
			}
		})
	}
}

func TestRecordArchitectureReviewBindsDigestAndKeepsHistory(t *testing.T) {
	doc := validArchitectureDocument()
	var stored model.IssueOpsRecord
	store := reviewport.ArchitectureReviewStore{
		Read:          func(_, _ string) (model.IssueOpsRecord, error) { return stored, nil },
		PlanReadiness: func(model.IssueOpsRecord) model.IssueOpsReadiness { return model.IssueOpsReadiness{Ready: true} },
		Now:           func() string { return "2026-09-28T00:00:00Z" },
		Document:      func(string, model.IssueOpsRecord) (string, error) { return doc, nil },
		TouchWrite: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			stored = record
			return record, nil
		},
	}
	stored = model.IssueOpsRecord{ID: "io-1"}
	if _, err := RecordArchitectureReview(store, "state", "io-1", approvedArchitectureRequest()); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256([]byte(doc))
	first := stored.ArchitectureReview
	if first.ReviewedArchitectureDigest != hex.EncodeToString(want[:]) || first.ReviewerContext != "subagent" || len(first.History) != 0 {
		t.Fatalf("first record must bind the document sha256 without history: %+v", first)
	}
	doc += "추가\n"
	if _, err := RecordArchitectureReview(store, "state", "io-1", approvedArchitectureRequest()); err != nil {
		t.Fatal(err)
	}
	second := stored.ArchitectureReview
	if len(second.History) != 1 || second.History[0].ReviewedArchitectureDigest != first.ReviewedArchitectureDigest ||
		second.ReviewedArchitectureDigest == first.ReviewedArchitectureDigest {
		t.Fatalf("re-recording must append the previous round and rebind the digest: %+v", second)
	}
}
