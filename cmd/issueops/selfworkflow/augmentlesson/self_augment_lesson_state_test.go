package augmentlesson

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	statestore "issueops/internal/adapter/outbound/state"
	augmentcontract "issueops/internal/contract/selfaugment"
)

func TestSaveSelfAugmentLesson(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
	result, err := SaveSelfAugmentLesson(augmentcontract.SelfAugmentLessonRequest{
		CandidateID: "reflexion-state-memory",
		Lesson:      "실패 교훈은 다음 cycle에서 재사용 가능해야 한다.",
		NextAction:  "다음 자가 증강 후보 선택 전에 저장된 lesson을 확인한다.",
		Source:      "unit-test",
		Severity:    "warning",
		StateKey:    "self-augment-lesson-test",
	}, lessonTestDeps{})
	if err != nil {
		t.Fatalf("SaveSelfAugmentLesson: %v", err)
	}
	if !result.OK || result.Kind != augmentcontract.SelfAugmentationLessonKind || result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("unexpected lesson result: %+v", result)
	}
	state, err := statestore.StateRead("self-augment-lesson-test")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	var snapshot augmentcontract.SelfAugmentLessonStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		t.Fatalf("unmarshal saved lesson snapshot: %v", err)
	}
	if snapshot.Kind != augmentcontract.SelfAugmentationLessonKind || snapshot.CandidateID != "reflexion-state-memory" || snapshot.NextAction == "" {
		t.Fatalf("unexpected lesson snapshot: %+v", snapshot)
	}
}

func TestSaveSelfAugmentLessonPrunesOldLessonRecords(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	if _, err := statestore.StateWrite(context.Background(), "self-augment-lesson-old", `{"kind":"self_augmentation_lesson"}`); err != nil {
		t.Fatalf("write old lesson: %v", err)
	}
	old, err := statestore.StateRead("self-augment-lesson-old")
	if err != nil {
		t.Fatalf("read old lesson: %v", err)
	}
	old.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	if _, err := statestore.WriteStateRecord(context.Background(), statestore.StateDir(), "self-augment-lesson-old", old.Record); err != nil {
		t.Fatalf("rewrite old lesson: %v", err)
	}

	if _, err := SaveSelfAugmentLesson(augmentcontract.SelfAugmentLessonRequest{
		CandidateID: "candidate-one",
		Lesson:      "old lessons should not grow forever",
		NextAction:  "keep only recent lesson state",
		Severity:    "error",
	}, lessonTestDeps{}); err != nil {
		t.Fatalf("SaveSelfAugmentLesson: %v", err)
	}

	if _, err := statestore.StateRead("self-augment-lesson-old"); err == nil {
		t.Fatalf("old lesson record should be pruned")
	}
}

func TestSaveSelfAugmentLessonRejectsMissingRequiredFields(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())

	if result, err := SaveSelfAugmentLesson(augmentcontract.SelfAugmentLessonRequest{CandidateID: "candidate-one"}, lessonTestDeps{}); err == nil || !strings.Contains(err.Error(), "lesson is required") || result.OK {
		t.Fatalf("expected missing lesson error, result=%#v err=%v", result, err)
	}
	if result, err := SaveSelfAugmentLesson(augmentcontract.SelfAugmentLessonRequest{CandidateID: "candidate-one", Lesson: "learned"}, lessonTestDeps{}); err == nil || !strings.Contains(err.Error(), "next-action is required") || result.OK {
		t.Fatalf("expected missing next-action error, result=%#v err=%v", result, err)
	}
}

func TestStateKeySlugNormalizesUnsafeText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "trims and lowercases", in: "  Ship This Lesson  ", want: "ship-this-lesson"},
		{name: "collapses punctuation", in: "A/B:C___D", want: "a-b-c-d"},
		{name: "fallback for empty slug", in: "!!!", want: "lesson"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StateKeySlug(tt.in)
			if got != tt.want {
				t.Fatalf("StateKeySlug(%q)=%q, want %q", tt.in, got, tt.want)
			}
			if strings.Contains(got, "_") {
				t.Fatalf("StateKeySlug(%q) kept underscore in %q", tt.in, got)
			}
		})
	}
}

func TestSaveSelfAugmentLessonAcceptsFreeSlugWhenCurriculumIsExhausted(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	deps := lessonTestDeps{SelectCandidate: func() *augmentcontract.SelfAugmentCandidate { return nil }}
	result, err := SaveSelfAugmentLesson(augmentcontract.SelfAugmentLessonRequest{
		CandidateID: "issueops-whoami-record-flags",
		Lesson:      "whoami must advertise both actor flag vectors",
		NextAction:  "rerun lifecycle dogfood",
	}, deps)
	if err != nil || !result.OK {
		t.Fatalf("free-slug lesson must be accepted when no open candidate exists: result=%#v err=%v", result, err)
	}
	if result.CandidateID != "issueops-whoami-record-flags" {
		t.Fatalf("candidate id = %q", result.CandidateID)
	}
}

func TestSaveSelfAugmentLessonRejectsInvalidFreeSlug(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	deps := lessonTestDeps{SelectCandidate: func() *augmentcontract.SelfAugmentCandidate { return nil }}
	for _, candidateID := range []string{"IssueOps Whoami", "issueops/whoami", "whoami:record"} {
		if result, err := SaveSelfAugmentLesson(augmentcontract.SelfAugmentLessonRequest{
			CandidateID: candidateID, Lesson: "l", NextAction: "n",
		}, deps); err == nil || result.OK {
			t.Fatalf("candidate id %q must be rejected, result=%#v err=%v", candidateID, result, err)
		}
	}
}
