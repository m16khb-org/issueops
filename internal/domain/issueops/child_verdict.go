package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

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

func ValidateChildValidationIDs(parentID, childID string) error {
	if parentID == "" {
		return fmt.Errorf("parent_id is required")
	}
	if childID == "" {
		return fmt.Errorf("child_id is required")
	}
	return nil
}

func ValidateChildForVerdict(child model.IssueOpsRecord, parentID, verdict string) error {
	if child.Delegation == nil || strings.TrimSpace(child.Delegation.ParentCycleID) != parentID {
		return fmt.Errorf("child_parent_mismatch: %s", child.ID)
	}
	if verdict == "accepted" && child.Phase != model.IssueOpsPhaseDone {
		return fmt.Errorf("child_not_done: %s", child.ID)
	}
	return nil
}

func ValidateArchivedChildObservation(child model.IssueOpsRecord, parentID string) error {
	if err := ValidateChildForVerdict(child, parentID, ""); err != nil {
		return err
	}
	return fmt.Errorf("child_reappeared: %s", child.ID)
}

func ApplyChildVerdict(parent, child model.IssueOpsRecord, verdict, reason string, evidence []string, now string) (model.IssueOpsRecord, model.IssueOpsChildCycleRef, error) {
	if strings.TrimSpace(parent.Repo) != strings.TrimSpace(child.Repo) {
		return parent, model.IssueOpsChildCycleRef{}, fmt.Errorf("child_repo_mismatch: %s", child.ID)
	}
	for _, ref := range parent.ChildCycles {
		if ref.CycleID == child.ID {
			return ApplyArchivedChildVerdict(parent, child.ID, verdict, reason, evidence, now)
		}
	}
	ref := childVerdictReceipt(ParentRefFromChild(child), verdict, reason, evidence, now)
	parent.ChildCycles = append(append([]model.IssueOpsChildCycleRef(nil), parent.ChildCycles...), ref)
	return parent, ref, nil
}

func ApplyArchivedChildVerdict(parent model.IssueOpsRecord, childID, verdict, reason string, evidence []string, now string) (model.IssueOpsRecord, model.IssueOpsChildCycleRef, error) {
	for i, ref := range parent.ChildCycles {
		if ref.CycleID != childID {
			continue
		}
		ref = childVerdictReceipt(ref, verdict, reason, evidence, now)
		parent.ChildCycles = append([]model.IssueOpsChildCycleRef(nil), parent.ChildCycles...)
		parent.ChildCycles[i] = ref
		return parent, ref, nil
	}
	return parent, model.IssueOpsChildCycleRef{}, fmt.Errorf("child_not_indexed: %s", childID)
}

func childVerdictReceipt(ref model.IssueOpsChildCycleRef, verdict, reason string, evidence []string, now string) model.IssueOpsChildCycleRef {
	ref.ValidationVerdict = verdict
	ref.ValidationReason = reason
	ref.ValidationEvidence = append([]string(nil), evidence...)
	ref.ValidatedAt = now
	return ref
}
