package selfaugment

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
)

func TestHistoryRejectsInvalidRequestBeforeStateRead(t *testing.T) {
	for _, tc := range []struct {
		limit     int
		retention contract.SelfAugmentHistoryRetentionOptions
		want      string
	}{
		{-1, contract.SelfAugmentHistoryRetentionOptions{Limit: -1}, "limit must be non-negative"},
		{0, contract.SelfAugmentHistoryRetentionOptions{Limit: -1}, "retention-limit must be non-negative"},
		{0, contract.SelfAugmentHistoryRetentionOptions{Confirm: true}, "confirm requires --prune-retention"},
		{0, contract.SelfAugmentHistoryRetentionOptions{PruneRequested: true}, "prune-retention requires a positive --retention-limit"},
	} {
		service := HistoryService{StateDir: func() string { return "/state" }, List: func() (state.StateListResult, error) {
			t.Fatal("invalid request reached state list")
			return state.StateListResult{}, nil
		}}
		result, err := service.History(context.Background(), "prefix", tc.limit, tc.retention)
		if err == nil || err.Error() != tc.want || result.OK || result.StateDir != "/state" || result.Entries == nil || result.Skipped == nil || result.Warnings == nil {
			t.Fatalf("result=%+v err=%v want=%s", result, err, tc.want)
		}
	}
}

func TestHistoryRetentionPreservesEffectOrderAndStopsAtFailure(t *testing.T) {
	for _, failureAt := range []string{"read:b", "delete:b", "read:c", "delete:c", ""} {
		t.Run(failureAt, func(t *testing.T) {
			failure := errors.New("storage unavailable")
			calls := []string{}
			service := HistoryService{
				Read: func(key string) (state.StateResult, error) {
					calls = append(calls, "read:"+key)
					if "read:"+key == failureAt {
						return state.StateResult{}, failure
					}
					return state.StateResult{}, nil
				},
				Delete: func(_ context.Context, key string) error {
					calls = append(calls, "delete:"+key)
					if "delete:"+key == failureAt {
						return failure
					}
					return nil
				},
			}
			result := contract.SelfAugmentHistoryResult{Entries: []contract.SelfAugmentHistoryEntry{{Key: "a"}, {Key: "b"}, {Key: "c"}}, TotalMatches: 3, Warnings: []string{}}
			err := service.ApplyRetention(context.Background(), &result, contract.SelfAugmentHistoryRetentionOptions{Limit: 1, PruneRequested: true, Confirm: true})
			sequence := []string{"read:b", "delete:b", "read:c", "delete:c"}
			want := sequence
			if failureAt != "" {
				for i, call := range sequence {
					if call == failureAt {
						want = sequence[:i+1]
						break
					}
				}
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatalf("calls=%v want=%v", calls, want)
			}
			if failureAt != "" {
				if !errors.Is(err, failure) || result.Retention != nil {
					t.Fatalf("partial failure receipt changed: %+v err=%v", result, err)
				}
			} else if err != nil || result.Retention == nil || !reflect.DeepEqual(result.Retention.DeletedKeys, []string{"b", "c"}) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestHistoryRetentionPreviewDoesNotTouchStorage(t *testing.T) {
	service := HistoryService{Read: func(string) (state.StateResult, error) {
		t.Fatal("preview read deletion candidate")
		return state.StateResult{}, nil
	}, Delete: func(context.Context, string) error { t.Fatal("preview deleted candidate"); return nil }}
	result := contract.SelfAugmentHistoryResult{Entries: []contract.SelfAugmentHistoryEntry{{Key: "a"}, {Key: "b"}}, TotalMatches: 2, Warnings: []string{}}
	if err := service.ApplyRetention(context.Background(), &result, contract.SelfAugmentHistoryRetentionOptions{Limit: 1, PruneRequested: true}); err != nil {
		t.Fatal(err)
	}
	if result.Retention == nil || !result.Retention.DryRun || len(result.Retention.DeletedKeys) != 0 || !reflect.DeepEqual(result.Retention.CandidateKeys, []string{"b"}) {
		t.Fatalf("result=%+v", result)
	}
}

func TestComparisonAdmissionAndReadOrder(t *testing.T) {
	failure := errors.New("cannot read")
	for _, tc := range []struct {
		base, candidate string
		threshold       float64
		failKey, want   string
		calls           []string
	}{
		{" ", "", -1, "", "baseline-key is required", nil},
		{"base", "", -1, "", "candidate-key is required", nil},
		{"base", "candidate", -1, "", "max elapsed regression pct must be non-negative", nil},
		{"base", "candidate", 5, "base", "read baseline summary: cannot read", []string{"base"}},
		{"base", "candidate", 5, "candidate", "read candidate summary: cannot read", []string{"base", "candidate"}},
	} {
		var calls []string
		service := HistoryService{StateDir: func() string { return "/state" }, Read: func(key string) (state.StateResult, error) {
			calls = append(calls, key)
			if key == tc.failKey {
				return state.StateResult{}, failure
			}
			return state.StateResult{Record: state.RecordEnvelope{Content: `{"schema_version":1,"kind":"self_verification_summary","summary":{"failure_cause":"none","failure_cause_reason":"no_failed_steps","failure_cause_evidence":[]}}`}}, nil
		}}
		result, err := service.Compare(tc.base, tc.candidate, tc.threshold)
		if err == nil || !strings.Contains(err.Error(), tc.want) || result.OK || !reflect.DeepEqual(calls, tc.calls) {
			t.Fatalf("calls=%v result=%+v err=%v want=%s", calls, result, err, tc.want)
		}
		if tc.failKey != "" && !errors.Is(err, failure) {
			t.Fatalf("lost storage cause: %v", err)
		}
	}
}

func TestHistoryCollectsDiagnosticsBeforeLimitingResults(t *testing.T) {
	contents := map[string]string{
		"keep-new":    `{"schema_version":1,"kind":"self_verification_summary","generated_at":"2026-09-29T00:00:00Z"}`,
		"keep-old":    `{"schema_version":1,"kind":"self_verification_summary","generated_at":"2026-09-28T00:00:00Z"}`,
		"keep-time":   `{"schema_version":1,"kind":"self_verification_summary","generated_at":"invalid"}`,
		"keep-schema": `{"schema_version":2,"kind":"self_verification_summary"}`,
		"keep-kind":   `{"schema_version":0,"kind":"other"}`,
		"keep-json":   "not JSON",
	}
	reads := []string{}
	service := HistoryService{
		StateDir: func() string { return "/state" },
		List: func() (state.StateListResult, error) {
			records := []state.StateListEntry{}
			for _, key := range []string{"unrelated", "keep-schema", "keep-old", "keep-time", "keep-new", "keep-missing", "keep-kind", "keep-json"} {
				records = append(records, state.StateListEntry{Key: key, UpdatedAt: "listed time", Bytes: 123})
			}
			return state.StateListResult{Records: records}, nil
		},
		Read: func(key string) (state.StateResult, error) {
			reads = append(reads, key)
			content, ok := contents[key]
			if !ok {
				return state.StateResult{}, errors.New("missing")
			}
			return state.StateResult{Record: state.RecordEnvelope{Content: content}}, nil
		},
		Delete: func(context.Context, string) error { t.Fatal("retention preview deleted state"); return nil },
	}
	result, err := service.History(context.Background(), "keep-", 1, contract.SelfAugmentHistoryRetentionOptions{Limit: 1, PruneRequested: true})
	if err != nil || !result.OK || result.TotalMatches != 3 || result.Returned != 1 || len(reads) != 7 || len(result.Entries) != 1 || result.Entries[0].Key != "keep-new" {
		t.Fatalf("result=%+v reads=%v err=%v", result, reads, err)
	}
	wantSkipped := []contract.SelfAugmentHistorySkipped{{Key: "keep-json", Reason: "not_json_summary"}, {Key: "keep-kind", Reason: "kind:other"}, {Key: "keep-missing", Reason: "state_read:missing"}, {Key: "keep-schema", Reason: "schema:2"}}
	wantWarnings := []string{"history_retention_candidates:2", "invalid_generated_at:keep-time"}
	if !reflect.DeepEqual(result.Skipped, wantSkipped) || !reflect.DeepEqual(result.Warnings, wantWarnings) || result.Retention == nil || !reflect.DeepEqual(result.Retention.CandidateKeys, []string{"keep-old", "keep-time"}) {
		t.Fatalf("classification/retention mismatch: %+v", result)
	}
	entry := result.Entries[0]
	if entry.StepLabels == nil || entry.SlowestSteps == nil || entry.UpdatedAt != "listed time" || entry.Bytes != 123 {
		t.Fatalf("entry metadata/empty arrays changed: %+v", entry)
	}
}
