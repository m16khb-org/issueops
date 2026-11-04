package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestAISlopCleanFingerprintReadinessMissingPreservesObservedCases(t *testing.T) {
	for _, tc := range []struct {
		name    string
		record  model.Record
		current string
		want    []string
	}{
		{"not cleaned", model.Record{AISlopCleanFingerprint: "old"}, "new", nil},
		{"both empty", model.Record{AISlopCleanAt: "now"}, "", nil},
		{"stored missing", model.Record{AISlopCleanAt: "now"}, "new", []string{"ai_slop_clean_fingerprint"}},
		{"current missing", model.Record{AISlopCleanAt: "now", AISlopCleanFingerprint: "old"}, "", []string{"current_fingerprint"}},
		{"stale", model.Record{AISlopCleanAt: "now", AISlopCleanFingerprint: "old"}, "new", []string{"ai_slop_clean_stale"}},
		{"same", model.Record{AISlopCleanAt: "now", AISlopCleanFingerprint: "same"}, "same", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := AISlopCleanFingerprintReadinessMissing(tc.record, tc.current); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("missing=%v, want %v", got, tc.want)
			}
		})
	}
}
