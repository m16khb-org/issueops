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
		Read: func(stateRoot, id string) (model.IssueOpsRecord, error) {
			if stateRoot != "state" || id != "io-1" {
				t.Fatalf("Read(%q, %q)", stateRoot, id)
			}
			return model.IssueOpsRecord{OK: true, ID: id}, nil
		},
		TouchWrite: func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
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
	record, err := RecordIntent(store, "state", "io-1", model.IssueOpsIntentRecordRequest{
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
	store := Store{Read: func(string, string) (model.IssueOpsRecord, error) {
		t.Fatal("invalid intent should fail before store read")
		return model.IssueOpsRecord{}, nil
	}}
	cases := []model.IssueOpsIntentRecordRequest{
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
		Document: func(string, model.IssueOpsRecord) (string, error) {
			return strings.Join(issueopsdomain.RequiredArchitectureSections, "\n내용\n") + "\n내용\n", nil
		},
		Read: func(stateRoot, id string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{OK: true, ID: id, Intent: &model.IssueOpsIntentContract{RawRequest: "raw"}}, nil
		},
		PlanReadiness: func(record model.IssueOpsRecord) model.IssueOpsReadiness {
			if record.Intent == nil {
				return model.IssueOpsReadiness{OK: true, Ready: false, Missing: []string{"intent"}}
			}
			return model.IssueOpsReadiness{OK: true, Ready: true}
		},
		TouchWrite: func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			if record.ArchitectureReview == nil {
				t.Fatal("architecture review was not recorded")
			}
			if !record.ArchitectureReview.Approved {
				t.Fatal("architecture review should be approved")
			}
			if got := record.ArchitectureReview.Verification; !stringSlicesEqual(got, []string{"go test", model.IssueOpsArchitectureReviewEvidenceExample}) {
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
		Verification:         []string{"go test", model.IssueOpsArchitectureReviewEvidenceExample},
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
		Read: func(stateRoot, id string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{OK: true, ID: id, Intent: &model.IssueOpsIntentContract{RawRequest: "raw"}}, nil
		},
		PlanReadiness: func(record model.IssueOpsRecord) model.IssueOpsReadiness {
			return model.IssueOpsReadiness{OK: true, Ready: true}
		},
		TouchWrite: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			t.Fatal("invalid architecture review should fail before write")
			return model.IssueOpsRecord{}, nil
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
	req.Verification = []string{model.IssueOpsArchitectureReviewEvidenceExample}
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
