package intentarchitecture

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
)

func TestRecordIntentWritesCleanRedactedContract(t *testing.T) {
	written := false
	store := Store{
		Read: func(stateRoot, id string) (model.Record, error) {
			if stateRoot != "state" || id != "io-1" {
				t.Fatalf("Read(%q, %q)", stateRoot, id)
			}
			return model.Record{OK: true, ID: id}, nil
		},
		TouchWrite: func(stateRoot string, record model.Record) (model.Record, error) {
			written = true
			if stateRoot != "state" {
				t.Fatalf("TouchWrite stateRoot = %q", stateRoot)
			}
			if record.Intent == nil {
				t.Fatal("intent was not recorded")
			}
			if got := record.Intent.SuccessCriteria; !stringSlicesEqual(got, []string{"tests pass", "docs updated"}) {
				t.Fatalf("success criteria = %#v", got)
			}
			if got := record.Intent.Constraints; !stringSlicesEqual(got, []string{"no schema changes"}) {
				t.Fatalf("constraints = %#v", got)
			}
			return record, nil
		},
	}
	record, err := RecordIntent(store, "state", "io-1", model.IntentRecordRequest{
		RawRequest:        "fix flaky quality gate in cmd/issueops",
		InterpretedIntent: "stabilize quality gate coverage workflow for harness commands",
		SuccessCriteria:   []string{" tests pass ", "", "docs updated", "tests pass"},
		Constraints:       []string{"no schema changes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !written || record.Intent == nil {
		t.Fatalf("record = %#v written=%v", record, written)
	}
}

func TestRecordIntentRejectsWeakContractsBeforeRead(t *testing.T) {
	store := Store{Read: func(string, string) (model.Record, error) {
		t.Fatal("invalid intent should fail before store read")
		return model.Record{}, nil
	}}
	cases := []model.IntentRecordRequest{
		{InterpretedIntent: "intent", SuccessCriteria: []string{"pass"}},
		{RawRequest: "raw", SuccessCriteria: []string{"pass"}},
		{RawRequest: "same request", InterpretedIntent: "same request", SuccessCriteria: []string{"pass"}},
		{RawRequest: "add login feature for all users", InterpretedIntent: "add login feature for all users", SuccessCriteria: []string{"pass"}},
		{RawRequest: "raw request with enough detail", InterpretedIntent: "different interpreted outcome"},
	}
	for _, req := range cases {
		if _, err := RecordIntent(store, "state", "io-1", req); err == nil {
			t.Fatalf("RecordIntent(%#v) succeeded", req)
		}
	}
}

func TestRecordArchitectureReviewWritesApprovedReview(t *testing.T) {
	store := Store{
		Document: func(string, model.Record) (string, error) {
			return strings.Join(issueopsdomain.RequiredArchitectureSections, "\n내용\n") + "\n내용\n", nil
		},
		Read: func(stateRoot, id string) (model.Record, error) {
			return model.Record{OK: true, ID: id, Intent: &model.IntentContract{RawRequest: "raw"}}, nil
		},
		PlanReadiness: func(record model.Record) model.Readiness {
			if record.Intent == nil {
				return model.Readiness{OK: true, Ready: false, Missing: []string{"intent"}}
			}
			return model.Readiness{OK: true, Ready: true}
		},
		TouchWrite: func(stateRoot string, record model.Record) (model.Record, error) {
			if record.ArchitectureReview == nil {
				t.Fatal("architecture review was not recorded")
			}
			if !record.ArchitectureReview.Approved {
				t.Fatal("architecture review should be approved")
			}
			if got := record.ArchitectureReview.Verification; !stringSlicesEqual(got, []string{"go test", model.ArchitectureReviewEvidenceExample}) {
				t.Fatalf("verification = %#v", got)
			}
			return record, nil
		},
	}
	record, err := RecordArchitectureReview(store, "state", "io-1", reviewcontract.ArchitectureReviewRequest{
		ProblemSummary:       "quality signal is low",
		ProposedArchitecture: "add focused tests",
		RefactorPlan:         "keep production unchanged",
		Alternatives:         []string{"raise threshold", "ignore package"},
		Risks:                []string{"brittle tests"},
		Verification:         []string{"go test", model.ArchitectureReviewEvidenceExample},
		Approved:             true,
		ReviewerContext:      "subagent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.ArchitectureReview == nil {
		t.Fatalf("record = %#v", record)
	}
}

func TestRecordArchitectureReviewRejectsIncompleteApprovedReview(t *testing.T) {
	store := Store{
		Read: func(stateRoot, id string) (model.Record, error) {
			return model.Record{OK: true, ID: id, Intent: &model.IntentContract{RawRequest: "raw"}}, nil
		},
		PlanReadiness: func(record model.Record) model.Readiness {
			return model.Readiness{OK: true, Ready: true}
		},
		TouchWrite: func(string, model.Record) (model.Record, error) {
			t.Fatal("invalid architecture review should fail before write")
			return model.Record{}, nil
		},
	}
	req := reviewcontract.ArchitectureReviewRequest{
		ProblemSummary:       "quality signal is low",
		ProposedArchitecture: "add focused tests",
		RefactorPlan:         "keep production unchanged",
		Alternatives:         []string{"raise threshold"},
		Risks:                []string{"brittle tests"},
		Verification:         []string{"go test"},
		Approved:             true,
	}
	if _, err := RecordArchitectureReview(store, "state", "io-1", req); err == nil {
		t.Fatal("approved architecture review without architecture review evidence should fail")
	}
	req.Verification = []string{model.ArchitectureReviewEvidenceExample}
	req.OpenQuestions = []string{"what next"}
	if _, err := RecordArchitectureReview(store, "state", "io-1", req); err == nil {
		t.Fatal("approved architecture review with open questions should fail")
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
