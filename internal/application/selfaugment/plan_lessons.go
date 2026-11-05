package selfaugment

import (
	"encoding/json"
	"strings"
	"time"

	contract "issueops/internal/contract/selfaugment"
	domain "issueops/internal/domain/selfaugment"
)

const selfAugmentLessonKeyPrefix = "self-augment-lesson-"

// LessonsCaptured reports whether any self-augment lesson is saved. A state
// listing failure counts as none.
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

// SevereLessonCountsAt counts, per candidate ID, the saved lessons that still
// penalize that candidate at now. Unreadable or malformed lessons are skipped;
// a failed state listing is returned as a warning.
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
		var snapshot contract.LessonStateSnapshot
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
