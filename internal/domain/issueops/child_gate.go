package issueops

import (
	"sort"
	"strings"

	model "issueops/internal/contract/issueops"
)

// ChildPRGateMissing lists the children that block the parent PR gate as
// "<reason>:<cycle id>" entries: children not done, not validated, or
// rejected without resolution. Dropped children are ignored.
func ChildPRGateMissing(children []model.ChildStatusEntry) []string {
	missing := []string{}
	for _, child := range children {
		if key := childPRGateKey(child); key != "" {
			missing = append(missing, key+":"+child.CycleID)
		}
	}
	return missing
}

// ActiveChildIDs returns the sorted cycle IDs of children that are neither
// dropped nor terminal.
func ActiveChildIDs(children []model.ChildStatusEntry) []string {
	ids := []string{}
	for _, child := range children {
		if !childDropped(child) && !childTerminal(child) {
			ids = append(ids, child.CycleID)
		}
	}
	sort.Strings(ids)
	return ids
}

func childPRGateKey(entry model.ChildStatusEntry) string {
	if childDropped(entry) {
		return ""
	}
	if !childTerminal(entry) {
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

func childTerminal(entry model.ChildStatusEntry) bool {
	return !entry.Orphaned && entry.Phase == model.PhaseDone
}

func childDropped(entry model.ChildStatusEntry) bool {
	return strings.TrimSpace(entry.ValidationVerdict) == "dropped" &&
		len(strings.TrimSpace(entry.ValidationReason)) >= 10 &&
		strings.TrimSpace(entry.ValidatedAt) != ""
}
