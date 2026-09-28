package issueops

import (
	"sort"
	"strings"

	model "issueops/internal/contract/issueops"
)

func ChildPRGateMissing(children []model.IssueOpsChildStatusEntry) []string {
	missing := []string{}
	for _, child := range children {
		if key := ChildPRGateKey(child); key != "" {
			missing = append(missing, key+":"+child.CycleID)
		}
	}
	return missing
}

func ActiveChildIDs(children []model.IssueOpsChildStatusEntry) []string {
	ids := []string{}
	for _, child := range children {
		if !ChildDropped(child) && !ChildTerminal(child) {
			ids = append(ids, child.CycleID)
		}
	}
	sort.Strings(ids)
	return ids
}

func ChildPRGateKey(entry model.IssueOpsChildStatusEntry) string {
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

func ChildTerminal(entry model.IssueOpsChildStatusEntry) bool {
	return !entry.Orphaned && entry.Phase == model.IssueOpsPhaseDone
}

func ChildDropped(entry model.IssueOpsChildStatusEntry) bool {
	return strings.TrimSpace(entry.ValidationVerdict) == "dropped" &&
		len(strings.TrimSpace(entry.ValidationReason)) >= 10 &&
		strings.TrimSpace(entry.ValidatedAt) != ""
}
