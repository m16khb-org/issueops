package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

// PrepareChildVerdict normalizes a validation verdict. An accepted verdict
// requires evidence and carries no reason; any other verdict requires a
// reason of at least 10 characters, and dropped discards evidence. It returns
// the reason and cleaned evidence.
func PrepareChildVerdict(verdict, reason string, evidence []string) (string, []string, error) {
	reason = strings.TrimSpace(reason)
	evidence = cleanDelegationValues(evidence)
	if verdict == "accepted" {
		if len(evidence) == 0 {
			return "", nil, fmt.Errorf("validation_evidence is required")
		}
		return "", evidence, nil
	}
	if len(reason) < 10 {
		return "", nil, fmt.Errorf("reason must be at least 10 characters")
	}
	if verdict == "dropped" {
		evidence = nil
	}
	return reason, evidence, nil
}

// ValidateChildValidationIDs requires both the parent and the child cycle ID
// of a validation request.
func ValidateChildValidationIDs(parentID, childID string) error {
	if parentID == "" {
		return fmt.Errorf("parent_id is required (--parent: the parent cycle's io-… id)")
	}
	if childID == "" {
		return fmt.Errorf("child_id is required")
	}
	return nil
}

// ValidateChildForVerdict checks that child belongs to parentID and, for an
// accepted verdict, is done.
func ValidateChildForVerdict(child model.Record, parentID, verdict string) error {
	if child.Delegation == nil || strings.TrimSpace(child.Delegation.ParentCycleID) != parentID {
		return fmt.Errorf("child_parent_mismatch: %s", child.ID)
	}
	if verdict == "accepted" && child.Phase != model.PhaseDone {
		return fmt.Errorf("child_not_done: %s", child.ID)
	}
	return nil
}

// ValidateArchivedChildObservation always fails: a child that belongs to
// parentID but reappears after its record was archived must be reconciled. A
// parent mismatch is reported first.
func ValidateArchivedChildObservation(child model.Record, parentID string) error {
	if err := ValidateChildForVerdict(child, parentID, ""); err != nil {
		return err
	}
	return fmt.Errorf("child_reappeared: %s", child.ID)
}

// ApplyChildVerdict stores a verdict receipt for child on parent. It rejects
// a child from another repository, updates the existing index entry when the
// child is already indexed, and otherwise appends a new entry.
func ApplyChildVerdict(parent, child model.Record, verdict, reason string, evidence []string, now string) (model.Record, model.ChildCycleRef, error) {
	if strings.TrimSpace(parent.Repo) != strings.TrimSpace(child.Repo) {
		return parent, model.ChildCycleRef{}, fmt.Errorf("child_repo_mismatch: %s", child.ID)
	}
	for _, ref := range parent.ChildCycles {
		if ref.CycleID == child.ID {
			return ApplyArchivedChildVerdict(parent, child.ID, verdict, reason, evidence, now)
		}
	}
	ref := childVerdictReceipt(parentRefFromChild(child), verdict, reason, evidence, now)
	parent.ChildCycles = append(append([]model.ChildCycleRef(nil), parent.ChildCycles...), ref)
	return parent, ref, nil
}

// ApplyArchivedChildVerdict stores a verdict receipt on the existing index
// entry of childID. It fails when the child is not indexed.
func ApplyArchivedChildVerdict(parent model.Record, childID, verdict, reason string, evidence []string, now string) (model.Record, model.ChildCycleRef, error) {
	for i, ref := range parent.ChildCycles {
		if ref.CycleID != childID {
			continue
		}
		ref = childVerdictReceipt(ref, verdict, reason, evidence, now)
		parent.ChildCycles = append([]model.ChildCycleRef(nil), parent.ChildCycles...)
		parent.ChildCycles[i] = ref
		return parent, ref, nil
	}
	return parent, model.ChildCycleRef{}, fmt.Errorf("child_not_indexed: %s", childID)
}

func childVerdictReceipt(ref model.ChildCycleRef, verdict, reason string, evidence []string, now string) model.ChildCycleRef {
	ref.ValidationVerdict = verdict
	ref.ValidationReason = reason
	ref.ValidationEvidence = append([]string(nil), evidence...)
	ref.ValidatedAt = now
	return ref
}
