package classification

import "sort"

type FindingKey struct {
	Index               int
	FailureClass        string
	FailureCause        string
	RecurringPattern    string
	ProposedKnob        string
	OverfitRisk         string
	VerificationCommand string
}

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
