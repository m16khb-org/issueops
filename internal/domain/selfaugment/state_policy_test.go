package selfaugment

import (
	"reflect"
	"strings"
	"testing"
	"time"

	contract "issueops/internal/contract/selfaugment"
)

func TestPrepareLessonValidatesSlugAndPreservesPartialResult(t *testing.T) {
	for _, tc := range []struct {
		id, lesson, next, wantError, wantLesson string
	}{
		{"", "l", "n", "candidate is required", ""},
		{"Bad", "l", "n", "must be kebab-case", ""},
		{"a/b", "l", "n", "must be kebab-case", ""},
		{"한글", "l", "n", "must be kebab-case", ""},
		{strings.Repeat("a", 97), "l", "n", "at most 96", ""},
		{"candidate", " ", "n", "lesson is required", ""},
		{"candidate", " l ", " ", "next-action is required", "l"},
		{"free_slug-1", " l ", " n ", "", "l"},
		{strings.Repeat("a", 96), "l", "n", "", "l"},
	} {
		t.Run(tc.id+tc.wantError, func(t *testing.T) {
			result, err := PrepareLesson(contract.SelfAugmentLessonRequest{Lesson: tc.lesson, NextAction: tc.next}, "/repo", tc.id)
			if (err == nil) != (tc.wantError == "") || (err != nil && !strings.Contains(err.Error(), tc.wantError)) || result.OK != (err == nil) || result.Lesson != tc.wantLesson || result.CandidateID != tc.id || result.IssueOpsRoot != "/repo" || result.GeneratedAt != "" || result.StateCheckpoint != nil {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestLessonSnapshotKeepsExplicitKeyAndNanosecondIdentity(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 1, time.UTC)
	result := contract.SelfAugmentLessonResult{CandidateID: "free_slug", GeneratedAt: now.Format(time.RFC3339Nano)}
	snapshot, key := LessonSnapshot(result, " custom-key ", now)
	if key != "custom-key" || snapshot.SchemaVersion != 1 || snapshot.GeneratedAt != result.GeneratedAt || snapshot.CandidateID != result.CandidateID {
		t.Fatalf("snapshot=%+v key=%q", snapshot, key)
	}
	_, first := LessonSnapshot(result, "", now)
	_, second := LessonSnapshot(result, "", now.Add(time.Nanosecond))
	if first != "self-augment-lesson-free-slug-20260102T030405Z-000000001" || second != "self-augment-lesson-free-slug-20260102T030405Z-000000002" {
		t.Fatalf("same-second identity: %q %q", first, second)
	}
}

func TestValidateSummarySnapshotRejectsRetiredKindAndUnsupportedSchema(t *testing.T) {
	for _, version := range []int{-1, 0, 1, 2} {
		for _, kind := range []string{"", "self_augment_summary", "self_verification_summary"} {
			err := ValidateSummarySnapshot("summary", contract.SelfAugmentStateSnapshot{SchemaVersion: version, Kind: kind})
			if (err == nil) != (version == 1 && kind == "self_verification_summary") {
				t.Fatalf("version=%d kind=%q err=%v", version, kind, err)
			}
		}
	}
}

func TestPlanSnapshotPreservesOrderAndEmptyLists(t *testing.T) {
	result := contract.SelfAugmentPlanResult{Candidates: []contract.SelfAugmentCandidate{{ID: "second", Status: contract.CandidateStatusOpen}, {ID: "first", Status: contract.CandidateStatusOpen}, {ID: "unknown", Status: "unknown"}}}
	snapshot := NewPlanSnapshot(result, time.Time{})
	if !reflect.DeepEqual(snapshot.OpenCandidateIDs, []string{"second", "first"}) || snapshot.SatisfiedCandidateIDs == nil || len(snapshot.SatisfiedCandidateIDs) != 0 || snapshot.CandidateCount != 3 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}
