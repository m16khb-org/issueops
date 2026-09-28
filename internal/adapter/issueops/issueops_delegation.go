package issueops

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

func IssueOpsChildStatus(stateRoot, parentID string, repair bool) (issueops.IssueOpsChildStatusResult, error) {
	return issueOpsChildStatus(stateRoot, parentID, repair, nil)
}

func IssueOpsChildStatusWithActor(stateRoot, parentID string, repair bool, actor IssueOpsActor) (issueops.IssueOpsChildStatusResult, error) {
	return issueOpsChildStatus(stateRoot, parentID, repair, &actor)
}

func issueOpsChildStatus(stateRoot, parentID string, repair bool, actor *IssueOpsActor) (issueops.IssueOpsChildStatusResult, error) {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		return issueops.IssueOpsChildStatusResult{OK: false}, fmt.Errorf("parent_id is required")
	}
	var parent issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, parentID, func(context.Context) error {
		var readErr error
		parent, readErr = ReadIssueOps(stateRoot, parentID)
		return readErr
	})
	if err != nil {
		return issueops.IssueOpsChildStatusResult{OK: false, ParentID: parentID}, err
	}

	scanned, err := scanIssueOpsChildrenForParent(stateRoot, parent)
	if err != nil {
		return issueops.IssueOpsChildStatusResult{OK: false, ParentID: parent.ID}, err
	}
	result := buildIssueOpsChildStatus(parent, scanned)
	if repair {
		appended, repairErr := repairIssueOpsChildIndex(stateRoot, parent.ID, scanned, actor)
		if repairErr != nil {
			return result, repairErr
		}
		result.RepairAppended = appended
		result.Repaired = len(appended) > 0
	}
	return result, nil
}

func AcceptIssueOpsChildWithActor(stateRoot, parentID, childID string, evidence []string, actor IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	return acceptIssueOpsChild(stateRoot, parentID, childID, evidence, &actor)
}

func acceptIssueOpsChild(stateRoot, parentID, childID string, evidence []string, actor *IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	evidence = cleanIssueOpsTextValues(evidence)
	if len(evidence) == 0 {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, fmt.Errorf("validation_evidence is required")
	}
	child, err := readIssueOpsChildForValidation(stateRoot, parentID, childID)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return acceptArchivedIssueOpsChild(stateRoot, parentID, childID, evidence, actor)
		}
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, err
	}
	if child.Phase != IssueOpsPhaseDone {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: child.ID}, fmt.Errorf("child_not_done: %s", child.ID)
	}
	return recordIssueOpsChildVerdict(stateRoot, parentID, child, "accepted", "", evidence, actor)
}

func acceptArchivedIssueOpsChild(stateRoot, parentID, childID string, evidence []string, actor *IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	parentID = strings.TrimSpace(parentID)
	childID = strings.TrimSpace(childID)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var updated issueops.IssueOpsChildCycleRef
	err := withIssueOpsLock(context.Background(), stateRoot, parentID, func(context.Context) error {
		parent, readErr := ReadIssueOps(stateRoot, parentID)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(parent, actor); actorErr != nil {
			return actorErr
		}
		if missingErr := ensureIssueOpsArchivedChildStillMissing(stateRoot, parentID, childID); missingErr != nil {
			return missingErr
		}
		for i := range parent.ChildCycles {
			if parent.ChildCycles[i].CycleID != childID {
				continue
			}
			parent.ChildCycles[i].ValidationVerdict = "accepted"
			parent.ChildCycles[i].ValidationReason = ""
			parent.ChildCycles[i].ValidationEvidence = evidence
			parent.ChildCycles[i].ValidatedAt = now
			updated = parent.ChildCycles[i]
			_, writeErr := touchAndWriteIssueOps(stateRoot, parent)
			return writeErr
		}
		return fmt.Errorf("child_not_indexed: %s", childID)
	})
	if err != nil {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: parentID, ChildID: childID}, err
	}
	return issueops.IssueOpsChildValidationResult{OK: true, ParentID: parentID, ChildID: childID, ParentRef: updated}, nil
}

func RejectIssueOpsChildWithActor(stateRoot, parentID, childID, reason string, evidence []string, actor IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	return rejectIssueOpsChild(stateRoot, parentID, childID, reason, evidence, &actor)
}

func rejectIssueOpsChild(stateRoot, parentID, childID, reason string, evidence []string, actor *IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) < 10 {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, fmt.Errorf("reason must be at least 10 characters")
	}
	child, err := readIssueOpsChildForValidation(stateRoot, parentID, childID)
	if err != nil {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, err
	}
	return recordIssueOpsChildVerdict(stateRoot, parentID, child, "rejected", reason, cleanIssueOpsTextValues(evidence), actor)
}

func DropIssueOpsChildWithActor(stateRoot, parentID, childID, reason string, actor IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	return dropIssueOpsChild(stateRoot, parentID, childID, reason, &actor)
}

func dropIssueOpsChild(stateRoot, parentID, childID, reason string, actor *IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) < 10 {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, fmt.Errorf("reason must be at least 10 characters")
	}
	child, err := readIssueOpsChildForValidation(stateRoot, parentID, childID)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return dropArchivedIssueOpsChild(stateRoot, parentID, childID, reason, actor)
		}
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: strings.TrimSpace(childID)}, err
	}
	return recordIssueOpsChildVerdict(stateRoot, parentID, child, "dropped", reason, nil, actor)
}

func dropArchivedIssueOpsChild(stateRoot, parentID, childID, reason string, actor *IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	parentID = strings.TrimSpace(parentID)
	childID = strings.TrimSpace(childID)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var updated issueops.IssueOpsChildCycleRef
	err := withIssueOpsLock(context.Background(), stateRoot, parentID, func(context.Context) error {
		parent, readErr := ReadIssueOps(stateRoot, parentID)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(parent, actor); actorErr != nil {
			return actorErr
		}
		if missingErr := ensureIssueOpsArchivedChildStillMissing(stateRoot, parentID, childID); missingErr != nil {
			return missingErr
		}
		for i := range parent.ChildCycles {
			if parent.ChildCycles[i].CycleID != childID {
				continue
			}
			parent.ChildCycles[i].ValidationVerdict = "dropped"
			parent.ChildCycles[i].ValidationReason = reason
			parent.ChildCycles[i].ValidationEvidence = nil
			parent.ChildCycles[i].ValidatedAt = now
			updated = parent.ChildCycles[i]
			_, writeErr := touchAndWriteIssueOps(stateRoot, parent)
			return writeErr
		}
		return fmt.Errorf("child_not_indexed: %s", childID)
	})
	if err != nil {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: parentID, ChildID: childID}, err
	}
	return issueops.IssueOpsChildValidationResult{OK: true, ParentID: parentID, ChildID: childID, ParentRef: updated}, nil
}

// 이미-held parent span은 state root의 mutation을 직렬화하므로 여기서 child lock을 다시 얻지 않는다.
func ensureIssueOpsArchivedChildStillMissing(stateRoot, parentID, childID string) error {
	child, err := ReadIssueOps(stateRoot, childID)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if child.Delegation == nil || strings.TrimSpace(child.Delegation.ParentCycleID) != parentID {
		return fmt.Errorf("child_parent_mismatch: %s", childID)
	}
	return fmt.Errorf("child_reappeared: %s", child.ID)
}

func scanIssueOpsChildrenForParent(stateRoot string, parent issueops.IssueOpsRecord) (map[string]issueops.IssueOpsRecord, error) {
	records, err := ScanReadableIssueOps(stateRoot)
	if err != nil {
		return nil, err
	}
	children := map[string]issueops.IssueOpsRecord{}
	for _, child := range records {
		if strings.TrimSpace(child.Repo) != strings.TrimSpace(parent.Repo) {
			continue
		}
		if child.Delegation == nil || strings.TrimSpace(child.Delegation.ParentCycleID) != parent.ID {
			continue
		}
		children[child.ID] = child
	}
	return children, nil
}

func buildIssueOpsChildStatus(parent issueops.IssueOpsRecord, scanned map[string]issueops.IssueOpsRecord) issueops.IssueOpsChildStatusResult {
	result := issueops.IssueOpsChildStatusResult{OK: true, ParentID: parent.ID}
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
	if parent.Phase == IssueOpsPhaseDone {
		for i := range result.Children {
			if issueopsdomain.ChildPRGateKey(result.Children[i]) != "" {
				result.Children[i].ParentClosedState = "parent_closed"
			}
		}
	}
	sort.Strings(result.Orphaned)
	return result
}

func repairIssueOpsChildIndex(stateRoot, parentID string, scanned map[string]issueops.IssueOpsRecord, actor *IssueOpsActor) ([]string, error) {
	appended := []string{}
	err := withIssueOpsLock(context.Background(), stateRoot, parentID, func(context.Context) error {
		parent, readErr := ReadIssueOps(stateRoot, parentID)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(parent, actor); actorErr != nil {
			return actorErr
		}
		seen := map[string]bool{}
		for _, ref := range parent.ChildCycles {
			seen[ref.CycleID] = true
		}
		childIDs := make([]string, 0, len(scanned))
		for childID := range scanned {
			childIDs = append(childIDs, childID)
		}
		sort.Strings(childIDs)
		for _, childID := range childIDs {
			if seen[childID] {
				continue
			}
			parent.ChildCycles = append(parent.ChildCycles, parentRefFromChild(scanned[childID]))
			appended = append(appended, childID)
		}
		if len(appended) == 0 {
			return nil
		}
		_, writeErr := touchAndWriteIssueOps(stateRoot, parent)
		return writeErr
	})
	return appended, err
}

func readIssueOpsChildForValidation(stateRoot, parentID, childID string) (issueops.IssueOpsRecord, error) {
	parentID = strings.TrimSpace(parentID)
	childID = strings.TrimSpace(childID)
	if parentID == "" {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("parent_id is required")
	}
	if childID == "" {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("child_id is required")
	}
	var child issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, childID, func(context.Context) error {
		var readErr error
		child, readErr = ReadIssueOps(stateRoot, childID)
		return readErr
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false, ID: childID}, err
	}
	if child.Delegation == nil || strings.TrimSpace(child.Delegation.ParentCycleID) != parentID {
		return child, fmt.Errorf("child_parent_mismatch: %s", childID)
	}
	return child, nil
}

func recordIssueOpsChildVerdict(stateRoot, parentID string, child issueops.IssueOpsRecord, verdict, reason string, evidence []string, actor *IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var updated issueops.IssueOpsChildCycleRef
	err := withIssueOpsLock(context.Background(), stateRoot, strings.TrimSpace(parentID), func(context.Context) error {
		parent, readErr := ReadIssueOps(stateRoot, parentID)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(parent, actor); actorErr != nil {
			return actorErr
		}
		if strings.TrimSpace(parent.Repo) != strings.TrimSpace(child.Repo) {
			return fmt.Errorf("child_repo_mismatch: %s", child.ID)
		}
		found := false
		for i := range parent.ChildCycles {
			if parent.ChildCycles[i].CycleID != child.ID {
				continue
			}
			parent.ChildCycles[i].ValidationVerdict = verdict
			parent.ChildCycles[i].ValidationReason = reason
			parent.ChildCycles[i].ValidationEvidence = evidence
			parent.ChildCycles[i].ValidatedAt = now
			updated = parent.ChildCycles[i]
			found = true
			break
		}
		if !found {
			updated = parentRefFromChild(child)
			updated.ValidationVerdict = verdict
			updated.ValidationReason = reason
			updated.ValidationEvidence = evidence
			updated.ValidatedAt = now
			parent.ChildCycles = append(parent.ChildCycles, updated)
		}
		_, writeErr := touchAndWriteIssueOps(stateRoot, parent)
		return writeErr
	})
	if err != nil {
		return issueops.IssueOpsChildValidationResult{OK: false, ParentID: strings.TrimSpace(parentID), ChildID: child.ID}, err
	}
	return issueops.IssueOpsChildValidationResult{OK: true, ParentID: strings.TrimSpace(parentID), ChildID: child.ID, ParentRef: updated}, nil
}

func childStatusEntryFromRef(ref issueops.IssueOpsChildCycleRef) issueops.IssueOpsChildStatusEntry {
	entry := issueops.IssueOpsChildStatusEntry{
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
		entry.Phase = IssueOpsPhaseDone
	}
	return entry
}

// issueOpsChildRefHasTerminalCleanupReceipt는 parent가 명시적으로 승인하거나
// 제외한 child만 cleanup 이후의 정상 레코드 부재로 판정한다.
func issueOpsChildRefHasTerminalCleanupReceipt(ref issueops.IssueOpsChildCycleRef) bool {
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

func childStatusEntryFromChild(child issueops.IssueOpsRecord) issueops.IssueOpsChildStatusEntry {
	entry := issueops.IssueOpsChildStatusEntry{CycleID: child.ID}
	mergeChildStatusRecord(&entry, child)
	return entry
}

func mergeChildStatusRecord(entry *issueops.IssueOpsChildStatusEntry, child issueops.IssueOpsRecord) {
	entry.CycleID = child.ID
	entry.Branch = child.Branch
	entry.Phase = child.Phase
	entry.LastActiveAt = LastActiveAt(child)
	entry.WorktreePath = strings.TrimSpace(child.WorktreePath)
	if child.Delegation != nil && entry.ChildIssueURL == "" {
		entry.ChildIssueURL = strings.TrimSpace(child.Delegation.ChildIssueURL)
	}
}

func parentRefFromChild(child issueops.IssueOpsRecord) issueops.IssueOpsChildCycleRef {
	ref := issueops.IssueOpsChildCycleRef{
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
