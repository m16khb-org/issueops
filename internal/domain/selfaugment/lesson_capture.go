package selfaugment

import (
	"fmt"
	"strings"
	"time"

	model "issueops/internal/contract/selfaugment"
)

// LessonStateKeyPrefix is the state key prefix under which self-augmentation lessons are stored.
const LessonStateKeyPrefix = "self-augment-lesson-"

// LessonStateMaxAge is how long a stored lesson remains eligible for retention.
const LessonStateMaxAge = 30 * 24 * time.Hour

// LessonStateMaxRecords caps how many lesson records are kept in state.
const LessonStateMaxRecords = 10000

const selfAugmentLessonCandidateIDMaxLen = 96

// validateLessonCandidateID는 소문자, 숫자, 하이픈, 밑줄로 된 후보 ID만 허용한다.
func validateLessonCandidateID(candidateID string) error {
	if len(candidateID) > selfAugmentLessonCandidateIDMaxLen {
		return fmt.Errorf("lesson candidate id must be at most %d characters", selfAugmentLessonCandidateIDMaxLen)
	}
	if strings.ContainsFunc(candidateID, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			return false
		default:
			return true
		}
	}) {
		return fmt.Errorf("lesson candidate id must be kebab-case (letters, digits, hyphen): %q", candidateID)
	}
	return nil
}

// PrepareLesson validates a lesson request and returns the normalized result: candidateID must be a
// lowercase slug, lesson and next action are required, and source and severity default to
// "self-augment" and "info". Failures return a not-OK result alongside the error.
func PrepareLesson(req model.LessonRequest, root, candidateID string) (model.LessonResult, error) {
	if candidateID == "" {
		return model.LessonResult{OK: false, Kind: model.SelfAugmentationLessonKind, LoopKind: "self_augmentation", KoreanName: model.SelfAugmentationKoreanName, IssueOpsRoot: root}, fmt.Errorf("candidate is required: pass --candidate ID (an existing curriculum candidate or a slug for dogfooding findings such as issueops-whoami-record-flags) because no selected open candidate is available")
	}
	// 후보 아이디는 커리큘럼 후보 또는 자유 dogfood 슬러그를 허용한다. 플래너가
	// 모든 후보를 already_satisfied로 소진한 상태에서도 lesson 캡처 경로가
	// 막히면 안 된다(SELF_AUGMENTATION.md 학습 캡처 계약).
	if err := validateLessonCandidateID(candidateID); err != nil {
		return model.LessonResult{OK: false, Kind: model.SelfAugmentationLessonKind, LoopKind: "self_augmentation", KoreanName: model.SelfAugmentationKoreanName, CandidateID: candidateID, IssueOpsRoot: root}, err
	}
	lesson := strings.TrimSpace(req.Lesson)
	if lesson == "" {
		return model.LessonResult{OK: false, Kind: model.SelfAugmentationLessonKind, LoopKind: "self_augmentation", KoreanName: model.SelfAugmentationKoreanName, CandidateID: candidateID, IssueOpsRoot: root}, fmt.Errorf("lesson is required")
	}
	nextAction := strings.TrimSpace(req.NextAction)
	if nextAction == "" {
		return model.LessonResult{OK: false, Kind: model.SelfAugmentationLessonKind, LoopKind: "self_augmentation", KoreanName: model.SelfAugmentationKoreanName, CandidateID: candidateID, Lesson: lesson, IssueOpsRoot: root}, fmt.Errorf("next-action is required")
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "self-augment"
	}
	severity := strings.TrimSpace(req.Severity)
	if severity == "" {
		severity = "info"
	}
	return model.LessonResult{
		OK: true, Kind: model.SelfAugmentationLessonKind, LoopKind: "self_augmentation", KoreanName: model.SelfAugmentationKoreanName,
		CandidateID: candidateID, Lesson: lesson, NextAction: nextAction, Source: source, Severity: severity, IssueOpsRoot: root,
	}, nil
}

// LessonSnapshot converts result into its state snapshot and returns it with the state key to
// write. A blank stateKey is replaced by a generated key whose nanosecond suffix keeps back-to-back
// lessons from colliding.
func LessonSnapshot(result model.LessonResult, stateKey string, now time.Time) (model.LessonStateSnapshot, string) {
	key := strings.TrimSpace(stateKey)
	if key == "" {
		// Nanosecond suffix: second-granularity keys collide when lessons are
		// recorded back-to-back in one loop, silently dropping earlier lessons.
		key = fmt.Sprintf("%s%s-%s-%09d", LessonStateKeyPrefix, StateKeySlug(result.CandidateID), now.Format("20060102T150405Z"), now.Nanosecond())
	}
	snapshot := model.LessonStateSnapshot{
		SchemaVersion: 1,
		Kind:          model.SelfAugmentationLessonKind,
		LoopKind:      result.LoopKind,
		KoreanName:    result.KoreanName,
		OK:            result.OK,
		CandidateID:   result.CandidateID,
		Lesson:        result.Lesson,
		NextAction:    result.NextAction,
		Source:        result.Source,
		Severity:      result.Severity,
		IssueOpsRoot:  result.IssueOpsRoot,
		GeneratedAt:   result.GeneratedAt,
	}
	return snapshot, key
}

// StateKeySlug lowercases s and collapses every run of characters outside [a-z0-9] into a single
// hyphen, trimming hyphens at both ends. It returns "lesson" when nothing remains.
func StateKeySlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "lesson"
	}
	return out
}
