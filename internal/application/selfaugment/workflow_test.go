package selfaugment

import (
	"errors"
	contract "issueops/internal/contract/selfaugment"
	"testing"
)

// Detects unconditional/missing saves, lost save errors, and discarded checkpoint mutations.
func TestPlanAndSave(t *testing.T) {
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
			result, err := PlanAndSave(contract.PlanRequest{Cycles: 2, TargetScore: 91}, tc.save, "custom-key", PlanAndSaveDeps{
				Plan: func(req contract.PlanRequest) contract.PlanResult {
					reads++
					if req.Cycles != 2 || req.TargetScore != 91 {
						t.Fatalf("request changed: %+v", req)
					}
					return contract.PlanResult{OK: true, KoreanName: "fixture"}
				},
				Save: func(result *contract.PlanResult, key string) error {
					writes++
					if reads != 1 || result.KoreanName != "fixture" || key != "custom-key" {
						t.Fatalf("incorrect save input: reads=%d result=%+v key=%q", reads, result, key)
					}
					result.StateCheckpoint = &contract.StateCheckpoint{OK: !tc.fail, Key: key}
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
