package selfverify

import (
	"errors"
	contract "issueops/internal/contract/selfaugment"
	"testing"
)

// Detects unconditional/missing saves, lost save errors, and discarded checkpoint mutations.
func TestExportAndSaveCandidates(t *testing.T) {
	failure := errors.New("storage unavailable")
	for _, tc := range []struct {
		name string
		save bool
		fail bool
	}{
		{"read-only", false, false}, {"saved", true, false}, {"save-failure", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads, writes := 0, 0
			result, err := ExportAndSaveCandidates(tc.save, "custom-key", ExportAndSaveCandidatesDeps{
				Export: func() contract.SelfVerificationCandidateExportResult {
					reads++

					return contract.SelfVerificationCandidateExportResult{OK: true, KoreanName: "fixture"}
				},
				Save: func(result *contract.SelfVerificationCandidateExportResult, key string) error {
					writes++
					if reads != 1 || result.KoreanName != "fixture" || key != "custom-key" {
						t.Fatalf("incorrect save input: reads=%d result=%+v key=%q", reads, result, key)
					}
					result.StateCheckpoint = &contract.SelfAugmentStateCheckpoint{OK: !tc.fail, Key: key}
					if tc.fail {
						result.StateCheckpoint.Error = failure.Error()
						return failure
					}
					return nil
				},
			})
			if reads != 1 || result.KoreanName != "fixture" {
				t.Fatalf("lost generated result: reads=%d result=%+v", reads, result)
			}
			if tc.save {
				if writes != 1 || result.StateCheckpoint == nil || result.StateCheckpoint.Key != "custom-key" || result.StateCheckpoint.OK == tc.fail {
					t.Fatalf("lost save outcome: writes=%d result=%+v", writes, result)
				}
			} else if writes != 0 || result.StateCheckpoint != nil {
				t.Fatalf("read-only operation saved: %+v", result)
			}
			if tc.fail {
				if !errors.Is(err, failure) {
					t.Fatalf("lost save error: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
