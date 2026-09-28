package issueopsreview

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func TestRecordAISlopCleanEvidenceRejectsBeforeSecondReadOrWrite(t *testing.T) {
	authorityErr := errors.New("foreign actor")
	for _, tc := range []struct {
		name       string
		categories []string
		verify     []string
		actorErr   error
		want       string
	}{
		{name: "actor", categories: nil, verify: nil, actorErr: authorityErr, want: "foreign actor"},
		{name: "category", categories: []string{"  "}, verify: []string{"go test"}, want: "at least one cleanup category"},
		{name: "verification", categories: []string{"dead-code"}, verify: nil, want: "at least one verification entry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads, writes, refreshes := 0, 0, 0
			store := reviewport.AISlopCleanStore{
				ReviewMutationStore: reviewport.ReviewMutationStore{
					WithLock: func(_, _ string, fn func() error) error { return fn() },
					Read: func(_, _ string) (model.IssueOpsRecord, error) {
						reads++
						return model.IssueOpsRecord{ID: "io-1"}, nil
					},
					ValidateMutation: func(model.IssueOpsRecord) error { return tc.actorErr },
					Write: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
						writes++
						return record, nil
					},
				},
				Refresh: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
					refreshes++
					return record, nil
				},
			}
			_, err := RecordAISlopCleanEvidence(store, "/state", "io-1", tc.categories, tc.verify)
			if err == nil || !strings.Contains(err.Error(), tc.want) || reads != 1 || writes != 0 || refreshes != 0 {
				t.Fatalf("err=%v reads=%d writes=%d refreshes=%d", err, reads, writes, refreshes)
			}
		})
	}
}

func TestRecordAISlopCleanEvidenceWritesNormalizedEvidenceOnce(t *testing.T) {
	reads, writes := 0, 0
	store := reviewport.AISlopCleanStore{
		ReviewMutationStore: reviewport.ReviewMutationStore{
			WithLock: func(_, _ string, fn func() error) error { return fn() },
			Read: func(_, _ string) (model.IssueOpsRecord, error) {
				reads++
				return model.IssueOpsRecord{ID: "io-1", Phase: model.IssueOpsPhaseImplement}, nil
			},
			ValidateMutation: func(model.IssueOpsRecord) error { return nil },
			Write: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
				writes++
				return record, nil
			},
			Now: func() string { return "2026-09-28T00:00:00Z" },
		},
		Refresh: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			t.Fatal("refresh should not run")
			return record, nil
		},
	}
	result, err := RecordAISlopCleanEvidence(store, "/state", "io-1", []string{" dead-code ", "", "dead-code", "duplication"}, []string{"go test"})
	if err != nil || reads != 2 || writes != 1 || !reflect.DeepEqual(result.AISlopCleanCategories, []string{"dead-code", "duplication"}) ||
		!reflect.DeepEqual(result.AISlopCleanVerification, []string{"go test"}) || result.UpdatedAt != "2026-09-28T00:00:00Z" {
		t.Fatalf("result=%+v err=%v reads=%d writes=%d", result, err, reads, writes)
	}
}

func TestRecordAISlopCleanEvidenceRefreshesCompletedPhase(t *testing.T) {
	reads, writes, refreshes := 0, 0, 0
	store := reviewport.AISlopCleanStore{
		ReviewMutationStore: reviewport.ReviewMutationStore{
			WithLock: func(_, _ string, fn func() error) error { return fn() },
			Read: func(_, _ string) (model.IssueOpsRecord, error) {
				reads++
				return model.IssueOpsRecord{ID: "io-1", Phase: model.IssueOpsPhaseFeedback, AISlopCleanAt: "already-cleaned"}, nil
			},
			ValidateMutation: func(model.IssueOpsRecord) error { return nil },
			Write: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
				writes++
				return record, nil
			},
		},
		Refresh: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			refreshes++
			if !reflect.DeepEqual(record.AISlopCleanCategories, []string{"dead-code"}) || !reflect.DeepEqual(record.AISlopCleanVerification, []string{"go test"}) {
				t.Fatalf("refresh record=%+v", record)
			}
			return record, nil
		},
	}
	_, err := RecordAISlopCleanEvidence(store, "/state", "io-1", []string{"dead-code"}, []string{"go test"})
	if err != nil || reads != 2 || writes != 0 || refreshes != 1 {
		t.Fatalf("err=%v reads=%d writes=%d refreshes=%d", err, reads, writes, refreshes)
	}
}
