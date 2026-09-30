package selfaugment

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
)

func TestPromoteBaselineGatesBeforeWriteAndRequiresReadback(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		ok, eligible, confirm, override bool
		failAt                          string
		wantEvents                      []string
		wantOK, wantPromoted            bool
	}{
		{"failed preview", false, false, false, false, "", []string{"source"}, true, false},
		{"failed source", false, true, true, false, "", []string{"source"}, false, false},
		{"ineligible source", true, false, true, false, "", []string{"source"}, false, false},
		{"override", false, false, true, true, "", []string{"source", "write", "readback"}, true, true},
		{"passing", true, true, true, false, "", []string{"source", "write", "readback"}, true, true},
		{"source error", true, true, true, false, "source", []string{"source"}, false, false},
		{"write error", true, true, true, false, "write", []string{"source", "write"}, false, false},
		{"readback error", true, true, true, false, "readback", []string{"source", "write", "readback"}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			fault := errors.New("unavailable")
			snapshot := contract.SelfAugmentStateSnapshot{OK: tc.ok, GeneratedAt: "source-time", Summary: contract.SelfAugmentSummary{TerminationEligible: tc.eligible}}
			result, err := PromoteBaseline("source", "baseline", tc.confirm, tc.override, PromoteDeps{
				StateDir: func() string { return "/state" },
				ReadSnapshot: func(key string) (contract.SelfAugmentStateSnapshot, error) {
					if key != "source" {
						t.Fatalf("unexpected source %q", key)
					}
					events = append(events, "source")
					if tc.failAt == "source" {
						return contract.SelfAugmentStateSnapshot{}, fault
					}
					return snapshot, nil
				},
				WriteSnapshot: func(dir, key string, got contract.SelfAugmentStateSnapshot) error {
					if dir != "/state" || key != "baseline" || !reflect.DeepEqual(got, snapshot) {
						t.Fatalf("unexpected write %q %q %+v", dir, key, got)
					}
					events = append(events, "write")
					if tc.failAt == "write" {
						return fault
					}
					return nil
				},
				ReadState: func(key string) (state.StateResult, error) {
					if key != "baseline" {
						t.Fatalf("unexpected readback %q", key)
					}
					events = append(events, "readback")
					if tc.failAt == "readback" {
						return state.StateResult{}, fault
					}
					return state.StateResult{Path: "/state/baseline", Record: state.RecordEnvelope{Bytes: 23}}, nil
				},
			})
			if !reflect.DeepEqual(events, tc.wantEvents) || result.OK != tc.wantOK || result.Promoted != tc.wantPromoted || (err == nil) != tc.wantOK {
				t.Fatalf("events=%v result=%+v err=%v", events, result, err)
			}
			if tc.failAt != "" && !errors.Is(err, fault) {
				t.Fatalf("lost cause: %v", err)
			}
			if tc.wantPromoted && (result.Path != "/state/baseline" || result.Bytes != 23) {
				t.Fatalf("missing receipt: %+v", result)
			}
			if !tc.wantPromoted && (result.Path != "" || result.Bytes != 0) {
				t.Fatalf("false receipt: %+v", result)
			}
		})
	}
}

func TestSavePlanSnapshotAndFailureCheckpoints(t *testing.T) {
	for _, failAt := range []string{"", "encode", "write"} {
		t.Run(failAt, func(t *testing.T) {
			fault := errors.New("unavailable")
			var written string
			result := contract.SelfAugmentPlanResult{OK: true, Candidates: []contract.SelfAugmentCandidate{{ID: "first", Status: contract.CandidateStatusOpen}, {ID: "done", Status: contract.CandidateStatusSatisfied}, {ID: "last", Status: contract.CandidateStatusOpen}}}
			err := SavePlan(&result, "", SavePlanDeps{
				Now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC) },
				Encode: func(s contract.SelfAugmentPlanStateSnapshot) ([]byte, error) {
					if failAt == "encode" {
						return nil, fault
					}
					return json.Marshal(s)
				},
				Write: func(key, content string) (state.StateResult, error) {
					if failAt == "encode" {
						t.Fatal("write after encoding failure")
					}
					if key != "self-augment-latest" {
						t.Fatalf("key=%q", key)
					}
					if failAt == "write" {
						return state.StateResult{}, fault
					}
					written = content
					return state.StateResult{StateDir: "/state", Path: "/state/plan", Record: state.RecordEnvelope{Key: key, Bytes: len(content)}}, nil
				},
				StateDir: func() string { return "/state" },
			})
			if failAt != "" {
				if !errors.Is(err, fault) || result.StateCheckpoint == nil || result.StateCheckpoint.OK || result.StateCheckpoint.Key != "self-augment-latest" || !result.OK {
					t.Fatalf("result=%+v err=%v", result, err)
				}
				if (result.StateCheckpoint.StateDir != "") != (failAt == "write") {
					t.Fatalf("checkpoint=%+v", result.StateCheckpoint)
				}
				return
			}
			var got contract.SelfAugmentPlanStateSnapshot
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(written), &got); err != nil {
				t.Fatal(err)
			}
			if got.SchemaVersion != 1 || got.Kind != "self_augmentation_plan" || got.GeneratedAt != "2026-01-02T03:04:05.000000006Z" || got.CandidateCount != 3 || !reflect.DeepEqual(got.OpenCandidateIDs, []string{"first", "last"}) || !reflect.DeepEqual(got.SatisfiedCandidateIDs, []string{"done"}) {
				t.Fatalf("snapshot=%+v", got)
			}
		})
	}
}

func TestSaveLessonStopsOnFailureAndPrunesOnlyAfterWrite(t *testing.T) {
	for _, failAt := range []string{"validation", "encode", "write", "prune", ""} {
		t.Run(failAt, func(t *testing.T) {
			fault := errors.New("unavailable")
			var events []string
			req := contract.SelfAugmentLessonRequest{CandidateID: "  free_slug  ", Lesson: " lesson ", NextAction: " next "}
			if failAt == "validation" {
				req.NextAction = " "
			}
			result, err := SaveLesson(req, SaveLessonDeps{
				SelectCandidate: func() *contract.SelfAugmentCandidate { t.Fatal("explicit candidate must skip selection"); return nil },
				Now:             func() time.Time { events = append(events, "clock"); return time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC) },
				Encode: func(s contract.SelfAugmentLessonStateSnapshot) ([]byte, error) {
					events = append(events, "encode")
					if failAt == "encode" {
						return nil, fault
					}
					return json.Marshal(s)
				},
				Write: func(key, content string) (state.StateResult, error) {
					events = append(events, "write")
					if key != "self-augment-lesson-free-slug-20260102T030405Z-000000006" {
						t.Fatalf("key=%s", key)
					}
					if !strings.Contains(content, `"candidate_id":"free_slug"`) {
						t.Fatalf("content=%s", content)
					}
					if failAt == "write" {
						return state.StateResult{}, fault
					}
					return state.StateResult{Record: state.RecordEnvelope{Key: key, Bytes: len(content)}}, nil
				},
				StateDir: func() string { return "/state" },
				Prune: func(prefix string, age time.Duration, max int, confirm bool) (state.StatePruneResult, error) {
					events = append(events, "prune")
					if prefix != "self-augment-lesson-" || age != 30*24*time.Hour || max != 10000 || !confirm {
						t.Fatalf("prune=%s %v %d %v", prefix, age, max, confirm)
					}
					if failAt == "prune" {
						return state.StatePruneResult{}, fault
					}
					return state.StatePruneResult{}, nil
				},
			})
			want := map[string][]string{"validation": nil, "encode": {"clock", "encode"}, "write": {"clock", "encode", "write"}, "prune": {"clock", "encode", "write", "prune"}, "": {"clock", "encode", "write", "prune"}}[failAt]
			wantOK := failAt == "" || failAt == "prune"
			if !reflect.DeepEqual(events, want) || result.OK != wantOK || (err == nil) != wantOK {
				t.Fatalf("events=%v result=%+v err=%v", events, result, err)
			}
			if wantOK && (result.Lesson != "lesson" || result.NextAction != "next" || result.Source != "self-augment" || result.Severity != "info") {
				t.Fatalf("result=%+v", result)
			}
			if failAt == "validation" && result.StateCheckpoint != nil {
				t.Fatalf("validation wrote checkpoint: %+v", result)
			}
			if failAt == "encode" || failAt == "write" {
				if !errors.Is(err, fault) || result.StateCheckpoint == nil || result.StateCheckpoint.OK {
					t.Fatalf("result=%+v err=%v", result, err)
				}
			}
		})
	}
}
