package issueops

import (
	"sort"
	"strings"

	model "issueops/internal/contract/issueops"
)

func ChildPRGateMissing(children []model.ChildStatusEntry) []string {
	missing := []string{}
	for _, child := range children {
		if key := ChildPRGateKey(child); key != "" {
			missing = append(missing, key+":"+child.CycleID)
		}
	}
	return missing
}

func ActiveChildIDs(children []model.ChildStatusEntry) []string {
	ids := []string{}
	for _, child := range children {
		if !ChildDropped(child) && !ChildTerminal(child) {
			ids = append(ids, child.CycleID)
		}
	}
	sort.Strings(ids)
	return ids
}

func ChildPRGateKey(entry model.ChildStatusEntry) string {
	if ChildDropped(entry) {
		return ""
	}
	if !ChildTerminal(entry) {
		return "child_incomplete"
	}
	switch strings.TrimSpace(entry.ValidationVerdict) {
	case "":
		return "child_unvalidated"
	case "rejected":
		return "child_rejected_unresolved"
	default:
		return ""
	}
}

func ChildTerminal(entry model.ChildStatusEntry) bool {
	return !entry.Orphaned && entry.Phase == model.PhaseDone
}

func ChildDropped(entry model.ChildStatusEntry) bool {
	return strings.TrimSpace(entry.ValidationVerdict) == "dropped" &&
		len(strings.TrimSpace(entry.ValidationReason)) >= 10 &&
		strings.TrimSpace(entry.ValidatedAt) != ""
}
