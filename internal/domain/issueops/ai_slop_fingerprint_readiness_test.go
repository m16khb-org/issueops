package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestAISlopCleanFingerprintReadinessMissingPreservesObservedCases(t *testing.T) {
	for _, tc := range []struct {
		name    string
		record  model.IssueOpsRecord
		current string
		want    []string
	}{
		{"not cleaned", model.IssueOpsRecord{AISlopCleanFingerprint: "old"}, "new", nil},
		{"both empty", model.IssueOpsRecord{AISlopCleanAt: "now"}, "", nil},
		{"stored missing", model.IssueOpsRecord{AISlopCleanAt: "now"}, "new", []string{"ai_slop_clean_fingerprint"}},
		{"current missing", model.IssueOpsRecord{AISlopCleanAt: "now", AISlopCleanFingerprint: "old"}, "", []string{"current_fingerprint"}},
		{"stale", model.IssueOpsRecord{AISlopCleanAt: "now", AISlopCleanFingerprint: "old"}, "new", []string{"ai_slop_clean_stale"}},
		{"same", model.IssueOpsRecord{AISlopCleanAt: "now", AISlopCleanFingerprint: "same"}, "same", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := AISlopCleanFingerprintReadinessMissing(tc.record, tc.current); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("missing=%v, want %v", got, tc.want)
			}
		})
	}
}
