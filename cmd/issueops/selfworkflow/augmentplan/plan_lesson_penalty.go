package augmentplan

import (
	"encoding/json"
	"strings"
	"time"

	"issueops/cmd/issueops/selfworkflow/model"
	"issueops/internal/domain/selfaugment"
)

const selfAugmentLessonKeyPrefix = "self-augment-lesson-"

func severeLessonSeverity(severity string) bool {
	return selfaugment.SevereLessonSeverity(severity)
}

func severeLessonCounts() (map[string]int, []string) {
	return severeLessonCountsAt(time.Now().UTC())
}

func severeLessonCountsAt(now time.Time) (map[string]int, []string) {
	counts := map[string]int{}
	list, err := StateList()
	if err != nil {
		return counts, []string{"lesson scan: " + err.Error()}
	}
	for _, key := range list.Keys {
		if !strings.HasPrefix(key, selfAugmentLessonKeyPrefix) {
			continue
		}
		record, err := StateRead(key)
		if err != nil {
			continue
		}
		var snapshot model.SelfAugmentLessonStateSnapshot
		if err := json.Unmarshal([]byte(record.Record.Content), &snapshot); err != nil {
			continue
		}
		if snapshot.Kind != model.SelfAugmentationLessonKind {
			continue
		}
		if snapshot.CandidateID == "" || !severeLessonSeverity(snapshot.Severity) {
			continue
		}
		if !recentLesson(snapshot.GeneratedAt, now) {
			continue
		}
		counts[snapshot.CandidateID]++
	}
	return counts, nil
}

func recentLesson(generatedAt string, now time.Time) bool {
	return selfaugment.RecentLesson(generatedAt, now)
}

func applyLessonPenalties(candidates []model.SelfAugmentCandidate, severeCounts map[string]int) []string {
	return selfaugment.ApplyLessonPenalties(candidates, severeCounts)
}
