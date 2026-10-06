package augmentplan

import (
	"time"
)

func severeLessonCounts() (map[string]int, []string) { return severeLessonCountsAt(time.Now().UTC()) }
func severeLessonCountsAt(now time.Time) (map[string]int, []string) {
	return planner().SevereLessonCountsAt(now)
}
