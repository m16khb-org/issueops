package selfaugment

import (
	"fmt"
	"strings"
	"time"

	contract "issueops/internal/contract/selfaugment"
)

const (
	lessonPenaltyThreshold = 2
	lessonPenaltyPerSevere = 15.0
	recentLessonWindow     = 30 * 24 * time.Hour
)

func SevereLessonSeverity(severity string) bool {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "error", "major", "critical", "blocker":
		return true
	}
	return false
}

func RecentLesson(generatedAt string, now time.Time) bool {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(generatedAt))
	if err != nil {
		return false
	}
	if t.After(now) {
		return true
	}
	return now.Sub(t) <= recentLessonWindow
}

func ApplyLessonPenalties(candidates []contract.SelfAugmentCandidate, severeCounts map[string]int) []string {
	warnings := []string{}
	for i := range candidates {
		if candidates[i].Status != contract.CandidateStatusOpen {
			continue
		}
		count := severeCounts[candidates[i].ID]
		if count < lessonPenaltyThreshold {
			continue
		}
		before := candidates[i].Score
		after := before - float64(count)*lessonPenaltyPerSevere
		if after < 0 {
			after = 0
		}
		candidates[i].Score = after
		warnings = append(warnings, fmt.Sprintf(
			"lesson penalty: candidate %q score %.1f -> %.1f after %d severe lessons (advisory demotion; candidate stays open)",
			candidates[i].ID, before, after, count))
	}
	return warnings
}

func LessonPenalizesCandidate(snapshot contract.SelfAugmentLessonStateSnapshot, now time.Time) bool {
	return snapshot.Kind == contract.SelfAugmentationLessonKind && snapshot.CandidateID != "" && SevereLessonSeverity(snapshot.Severity) && RecentLesson(snapshot.GeneratedAt, now)
}
