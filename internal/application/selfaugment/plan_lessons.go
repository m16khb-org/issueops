package selfaugment

import (
	"encoding/json"
	"strings"
	"time"

	contract "issueops/internal/contract/selfaugment"
	domain "issueops/internal/domain/selfaugment"
)

const selfAugmentLessonKeyPrefix = "self-augment-lesson-"

func (planner Planner) LessonsCaptured() bool {
	list, err := planner.StateList()
	if err != nil {
		return false
	}
	for _, key := range list.Keys {
		if strings.HasPrefix(key, selfAugmentLessonKeyPrefix) {
			return true
		}
	}
	return false
}

func (planner Planner) SevereLessonCountsAt(now time.Time) (map[string]int, []string) {
	counts := map[string]int{}
	list, err := planner.StateList()
	if err != nil {
		return counts, []string{"lesson scan: " + err.Error()}
	}
	for _, key := range list.Keys {
		if !strings.HasPrefix(key, selfAugmentLessonKeyPrefix) {
			continue
		}
		record, err := planner.StateRead(key)
		if err != nil {
			continue
		}
		var snapshot contract.SelfAugmentLessonStateSnapshot
		if err := json.Unmarshal([]byte(record.Record.Content), &snapshot); err != nil {
			continue
		}
		if !domain.LessonPenalizesCandidate(snapshot, now) {
			continue
		}
		counts[snapshot.CandidateID]++
	}
	return counts, nil
}
