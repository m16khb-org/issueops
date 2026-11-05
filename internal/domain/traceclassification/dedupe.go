package traceclassification

import "sort"

// FindingKey is the comparable projection of one trace finding used for
// deduplication and ordering. Index points back at the finding's position in the
// caller's slice.
type FindingKey struct {
	Index               int
	FailureClass        string
	FailureCause        string
	RecurringPattern    string
	ProposedKnob        string
	OverfitRisk         string
	VerificationCommand string
}

// DeduplicateFindings drops findings that repeat the same failure class, cause,
// recurring pattern, and proposed knob, keeping the first occurrence. It returns
// the surviving Index values ordered by class, cause, pattern, knob, overfit risk,
// then verification command.
func DeduplicateFindings(findings []FindingKey) []int {
	seen := map[string]bool{}
	selected := []FindingKey{}
	for _, finding := range findings {
		key := finding.FailureClass + "\x00" + finding.FailureCause + "\x00" + finding.RecurringPattern + "\x00" + finding.ProposedKnob
		if seen[key] {
			continue
		}
		seen[key] = true
		selected = append(selected, finding)
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].FailureClass != selected[j].FailureClass {
			return selected[i].FailureClass < selected[j].FailureClass
		}
		if selected[i].FailureCause != selected[j].FailureCause {
			return selected[i].FailureCause < selected[j].FailureCause
		}
		if selected[i].RecurringPattern != selected[j].RecurringPattern {
			return selected[i].RecurringPattern < selected[j].RecurringPattern
		}
		if selected[i].ProposedKnob != selected[j].ProposedKnob {
			return selected[i].ProposedKnob < selected[j].ProposedKnob
		}
		if selected[i].OverfitRisk != selected[j].OverfitRisk {
			return selected[i].OverfitRisk < selected[j].OverfitRisk
		}
		return selected[i].VerificationCommand < selected[j].VerificationCommand
	})
	indices := make([]int, 0, len(selected))
	for _, finding := range selected {
		indices = append(indices, finding.Index)
	}
	return indices
}
