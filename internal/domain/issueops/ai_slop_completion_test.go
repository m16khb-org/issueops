package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestAISlopCleanCompletionMissingPreservesEvidenceKeys(t *testing.T) {
	got := AISlopCleanCompletionMissing(model.IssueOpsRecord{
		AISlopCleanAt:           "2026-09-28T00:00:00Z",
		AISlopCleanHead:         "head",
		AISlopCleanCategories:   []string{"\x00"},
		AISlopCleanVerification: []string{"  "},
	})
	want := []string{"ai_slop_clean_fingerprint", "cleanup_evidence", "verification_evidence"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("missing=%v, want %v", got, want)
	}
	complete := model.IssueOpsRecord{
		AISlopCleanAt: "now", AISlopCleanHead: "head", AISlopCleanFingerprint: "fingerprint",
		AISlopCleanCategories: []string{"cleanup"}, AISlopCleanVerification: []string{"go test"},
	}
	if got := AISlopCleanCompletionMissing(complete); len(got) != 0 {
		t.Fatalf("complete evidence missing=%v", got)
	}
}
