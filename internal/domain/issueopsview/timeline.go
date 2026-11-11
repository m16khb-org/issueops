// Package issueopsview holds the domain rules that shape the cycle views
// `issueops view` renders from a live record or a retained outcome.
package issueopsview

import (
	issueopsviewcontract "issueops/internal/contract/issueopsview"
)

// RetainedTimeline returns the timeline a retained outcome carries. An outcome
// written before timelines existed has none; it answers an empty timeline that
// names the gap instead of a nil a reader could mistake for "no events".
func RetainedTimeline(outcome issueopsviewcontract.CycleOutcome) issueopsviewcontract.CycleTimeline {
	if outcome.Timeline != nil {
		return *outcome.Timeline
	}
	return issueopsviewcontract.CycleTimeline{
		Limits: []string{issueopsviewcontract.TimelineLimitRetainedWithoutTimeline},
		Events: []issueopsviewcontract.TimelineEvent{},
	}
}
