package augmentplan

import (
	"time"

	contract "issueops/internal/contract/selfaugment"
	domain "issueops/internal/domain/selfaugment"
)

func severeLessonCounts() (map[string]int, []string) { return severeLessonCountsAt(time.Now().UTC()) }
func severeLessonCountsAt(now time.Time) (map[string]int, []string) {
	return planner().SevereLessonCountsAt(now)
}
func applyLessonPenalties(candidates []contract.SelfAugmentCandidate, counts map[string]int) []string {
	return domain.ApplyLessonPenalties(candidates, counts)
}
