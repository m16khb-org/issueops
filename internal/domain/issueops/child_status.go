package issueops

import (
	"sort"
	"strings"

	model "issueops/internal/contract/issueops"
)

func SelectChildren(parent model.Record, records []model.Record) map[string]model.Record {
	children := map[string]model.Record{}
	for _, child := range records {
		if strings.TrimSpace(child.Repo) == strings.TrimSpace(parent.Repo) &&
			child.Delegation != nil && strings.TrimSpace(child.Delegation.ParentCycleID) == parent.ID {
			children[child.ID] = child
		}
	}
	return children
}

func RepairChildIndex(parent model.Record, scanned map[string]model.Record) (model.Record, []string) {
	appended := []string{}
	seen := map[string]bool{}
	for _, ref := range parent.ChildCycles {
		seen[ref.CycleID] = true
	}
	for id := range scanned {
		if !seen[id] {
			appended = append(appended, id)
		}
	}
	sort.Strings(appended)
	if len(appended) == 0 {
		return parent, appended
	}
	parent.ChildCycles = append([]model.ChildCycleRef(nil), parent.ChildCycles...)
	for _, id := range appended {
		parent.ChildCycles = append(parent.ChildCycles, ParentRefFromChild(scanned[id]))
	}
	return parent, appended
}

func BuildChildStatus(parent model.Record, scanned map[string]model.Record) model.ChildStatusResult {
	result := model.ChildStatusResult{OK: true, ParentID: parent.ID}
	seen := map[string]bool{}
	for _, ref := range parent.ChildCycles {
		entry := childStatusEntryFromRef(ref)
		entry.Indexed = true
		if child, ok := scanned[ref.CycleID]; ok {
			mergeChildStatusRecord(&entry, child)
			entry.Scanned = true
		} else if !issueOpsChildRefHasTerminalCleanupReceipt(ref) {
			entry.Orphaned = true
			result.Orphaned = append(result.Orphaned, ref.CycleID)
		}
		result.Children = append(result.Children, entry)
		seen[ref.CycleID] = true
	}
	for _, child := range scanned {
		if seen[child.ID] {
			continue
		}
		entry := childStatusEntryFromChild(child)
		entry.Scanned = true
		result.Children = append(result.Children, entry)
	}
	sort.Slice(result.Children, func(i, j int) bool {
		return result.Children[i].CycleID < result.Children[j].CycleID
	})
	if parent.Phase == model.PhaseDone {
		for i := range result.Children {
			if ChildPRGateKey(result.Children[i]) != "" {
				result.Children[i].ParentClosedState = "parent_closed"
			}
		}
	}
	sort.Strings(result.Orphaned)
	return result
}

func childStatusEntryFromRef(ref model.ChildCycleRef) model.ChildStatusEntry {
	entry := model.ChildStatusEntry{
		CycleID:            ref.CycleID,
		Branch:             ref.Branch,
		Title:              ref.Title,
		ChildIssueURL:      ref.ChildIssueURL,
		ValidationVerdict:  ref.ValidationVerdict,
		ValidationReason:   ref.ValidationReason,
		ValidationEvidence: append([]string{}, ref.ValidationEvidence...),
		ValidatedAt:        ref.ValidatedAt,
	}
	// accepted는 done child만 기록할 수 있으므로 parent receipt 자체가 cleanup
	// 이후에도 terminal phase의 내구성 있는 증거다.
	if strings.TrimSpace(ref.ValidationVerdict) == "accepted" && strings.TrimSpace(ref.ValidatedAt) != "" {
		entry.Phase = model.PhaseDone
	}
	return entry
}

// issueOpsChildRefHasTerminalCleanupReceipt는 parent가 명시적으로 승인하거나
// 제외한 child만 cleanup 이후의 정상 레코드 부재로 판정한다.
func issueOpsChildRefHasTerminalCleanupReceipt(ref model.ChildCycleRef) bool {
	if strings.TrimSpace(ref.ValidatedAt) == "" {
		return false
	}
	switch strings.TrimSpace(ref.ValidationVerdict) {
	case "accepted":
		return len(ref.ValidationEvidence) > 0
	case "dropped":
		return len(strings.TrimSpace(ref.ValidationReason)) >= 10
	default:
		return false
	}
}

func childStatusEntryFromChild(child model.Record) model.ChildStatusEntry {
	entry := model.ChildStatusEntry{CycleID: child.ID}
	mergeChildStatusRecord(&entry, child)
	return entry
}

func mergeChildStatusRecord(entry *model.ChildStatusEntry, child model.Record) {
	entry.CycleID = child.ID
	entry.Branch = child.Branch
	entry.Phase = child.Phase
	entry.LastActiveAt = child.UpdatedAt
	if strings.TrimSpace(entry.LastActiveAt) == "" {
		entry.LastActiveAt = child.CreatedAt
	}
	entry.WorktreePath = strings.TrimSpace(child.WorktreePath)
	if child.Delegation != nil && entry.ChildIssueURL == "" {
		entry.ChildIssueURL = strings.TrimSpace(child.Delegation.ChildIssueURL)
	}
}

func ParentRefFromChild(child model.Record) model.ChildCycleRef {
	ref := model.ChildCycleRef{
		CycleID:   child.ID,
		Branch:    child.Branch,
		Title:     child.Branch,
		CreatedAt: child.CreatedAt,
	}
	if child.Delegation != nil {
		ref.ChildIssueURL = strings.TrimSpace(child.Delegation.ChildIssueURL)
		if delegatedAt := strings.TrimSpace(child.Delegation.DelegatedAt); delegatedAt != "" {
			ref.CreatedAt = delegatedAt
		}
	}
	return ref
}
