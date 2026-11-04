package issueops

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	model "issueops/internal/contract/issueops"
)

var errRetargetObservationStale = errors.New("IssueOps record changed while the retarget was observed; retry the command")

// RetargetRequiresHolder reports whether retargeting needs the execution
// lease holder: true when the record has an execution with an active lease. A
// record whose execution fails ValidateExecution is an error.
func RetargetRequiresHolder(record model.Record) (bool, error) {
	if record.Execution == nil {
		return false, nil
	}
	if err := ValidateExecution(*record.Execution); err != nil {
		return false, fmt.Errorf("invalid IssueOps execution v1 record: %w", err)
	}
	return record.Execution.Lease.Status == model.LeaseStatusActive, nil
}

// ValidateRetargetRequest requires the new base branch and a reason for a
// retarget.
func ValidateRetargetRequest(base, reason string) error {
	if base == "" {
		return fmt.Errorf("base_branch is required (--base-branch: the PR's new target branch)")
	}
	if reason == "" {
		return fmt.Errorf("reason is required (--reason TEXT)")
	}
	return nil
}

// ValidateRetargetRecord checks that the record can be retargeted to base:
// the branch is prepared, a verified remote artifact with a URL exists, and
// base differs from the prepared base.
func ValidateRetargetRecord(record model.Record, base string) error {
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

// ValidateRetargetArtifact returns a stale-observation error unless the
// artifact verification observed before the remote read still equals the
// current record value.
func ValidateRetargetArtifact(observed *model.RemoteArtifactVerification, current model.RemoteArtifactVerification) error {
	if observed == nil || !reflect.DeepEqual(*observed, current) {
		return errRetargetObservationStale
	}
	return nil
}

// ValidateRetargetTarget fails when the remote artifact reports a target
// branch other than the requested base.
func ValidateRetargetTarget(base, observed string) error {
	if strings.TrimSpace(observed) != base {
		return fmt.Errorf("remote artifact targets %q, not %q", observed, base)
	}
	return nil
}

// ValidateRetargetOrigin checks the origin observation behind a retarget. It
// fails when the observation was not stated or its repository or base differs
// from the request (stale), when the observation itself errored, or when base
// is absent from origin.
func ValidateRetargetOrigin(repo, base, observedRepo, observedBase string, stated, present bool, observationErr error) error {
	if !stated || repo != observedRepo || base != observedBase {
		return fmt.Errorf("origin observation failed: %w", errRetargetObservationStale)
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
func ApplyBranchRetarget(record model.Record, base, reason, observedAt, updatedAt string) model.Record {
	prepare := *record.BranchPrepare
	artifact := *record.RemoteArtifact
	prepare.Retargets = append(append([]model.BranchRetarget(nil), prepare.Retargets...), model.BranchRetarget{
		FromBase: strings.TrimSpace(prepare.BaseBranch), ToBase: base, Reason: reason,
		ArtifactURL: strings.TrimSpace(artifact.URL), ObservedAt: observedAt,
	})
	prepare.BaseBranch = base
	artifact.TargetBranch = base
	record.BranchPrepare, record.RemoteArtifact = &prepare, &artifact
	record.UpdatedAt = updatedAt
	return record
}
