package issueops

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	model "issueops/internal/contract/issueops"
)

var ErrRetargetObservationStale = errors.New("IssueOps record changed while the retarget was observed; retry the command")

func RetargetRequiresHolder(record model.IssueOpsRecord) (bool, error) {
	if record.Execution == nil {
		return false, nil
	}
	if err := ValidateExecution(*record.Execution); err != nil {
		return false, fmt.Errorf("invalid IssueOps execution v1 record: %w", err)
	}
	return record.Execution.Lease.Status == model.LeaseStatusActive, nil
}

func ValidateRetargetRequest(base, reason string) error {
	if base == "" {
		return fmt.Errorf("base_branch is required")
	}
	if reason == "" {
		return fmt.Errorf("reason is required")
	}
	return nil
}

func ValidateRetargetRecord(record model.IssueOpsRecord, base string) error {
	if record.BranchPrepare == nil {
		return fmt.Errorf("branch must be prepared before retarget")
	}
	if record.RemoteArtifact == nil || strings.TrimSpace(record.RemoteArtifact.URL) == "" {
		return fmt.Errorf("a verified remote artifact is required before retarget")
	}
	if strings.TrimSpace(record.BranchPrepare.BaseBranch) == base {
		return fmt.Errorf("base_branch %q is already the prepared base", base)
	}
	return nil
}

func ValidateRetargetArtifact(observed *model.IssueOpsRemoteArtifactVerification, current model.IssueOpsRemoteArtifactVerification) error {
	if observed == nil || !reflect.DeepEqual(*observed, current) {
		return ErrRetargetObservationStale
	}
	return nil
}

func ValidateRetargetTarget(base, observed string) error {
	if strings.TrimSpace(observed) != base {
		return fmt.Errorf("remote artifact targets %q, not %q", observed, base)
	}
	return nil
}

func ValidateRetargetOrigin(repo, base, observedRepo, observedBase string, stated, present bool, observationErr error) error {
	if !stated || repo != observedRepo || base != observedBase {
		return fmt.Errorf("origin observation failed: %w", ErrRetargetObservationStale)
	}
	if observationErr != nil {
		return fmt.Errorf("origin observation failed: %w", observationErr)
	}
	if !present {
		return fmt.Errorf("base_branch %q is absent from origin", base)
	}
	return nil
}

// ApplyBranchRetarget preserves the sealed fork point and the input snapshot.
func ApplyBranchRetarget(record model.IssueOpsRecord, base, reason, observedAt, updatedAt string) model.IssueOpsRecord {
	prepare := *record.BranchPrepare
	artifact := *record.RemoteArtifact
	prepare.Retargets = append(append([]model.IssueOpsBranchRetarget(nil), prepare.Retargets...), model.IssueOpsBranchRetarget{
		FromBase: strings.TrimSpace(prepare.BaseBranch), ToBase: base, Reason: reason,
		ArtifactURL: strings.TrimSpace(artifact.URL), ObservedAt: observedAt,
	})
	prepare.BaseBranch = base
	artifact.TargetBranch = base
	record.BranchPrepare, record.RemoteArtifact = &prepare, &artifact
	record.UpdatedAt = updatedAt
	return record
}
