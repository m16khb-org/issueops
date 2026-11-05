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

func severeLessonSeverity(severity string) bool {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "error", "major", "critical", "blocker":
		return true
	}
	return false
}

func recentLesson(generatedAt string, now time.Time) bool {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(generatedAt))
	if err != nil {
		return false
	}
	if t.After(now) {
		return true
	}
	return now.Sub(t) <= recentLessonWindow
}

// ApplyLessonPenalties demotes open candidates in place that have at least two severe lessons,
// reducing their score by 15 per lesson with a floor of zero. severeCounts maps candidate ID to
// severe lesson count; candidates stay open and one advisory warning is returned per demotion.
func ApplyLessonPenalties(candidates []contract.Candidate, severeCounts map[string]int) []string {
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

// LessonPenalizesCandidate reports whether snapshot counts as a severe lesson against its
// candidate: a self-augmentation lesson with a candidate ID, a severe severity, and a generation
// time within the last 30 days of now.
func LessonPenalizesCandidate(snapshot contract.LessonStateSnapshot, now time.Time) bool {
	return snapshot.Kind == contract.SelfAugmentationLessonKind && snapshot.CandidateID != "" && severeLessonSeverity(snapshot.Severity) && recentLesson(snapshot.GeneratedAt, now)
}
