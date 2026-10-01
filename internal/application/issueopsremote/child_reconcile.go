package issueopsremote

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	remote "issueops/internal/domain/issueopsremote"
	"issueops/internal/port"
	"strings"
)

type ChildReconcileCommand struct {
	ID, OperationID string
	Confirm         bool
	Actor           model.IssueOpsActor
}
type ChildReconciler struct {
	Records IssueRecordReader
	Resolve func(string) (port.IssueProviderChildCreateRecovery, error)
	Intents *ChildCreateIntents
}

func (s ChildReconciler) Reconcile(ctx context.Context, cmd ChildReconcileCommand, observe AncestryObserver) (model.ChildReconcileResult, error) {
	result := model.ChildReconcileResult{OperationID: cmd.OperationID, RecoveryCommand: ChildRecoveryCommand(cmd.ID, cmd.OperationID, cmd.Actor)}
	record, err := s.Records.Read(ctx, cmd.ID)
	if err != nil {
		return result, err
	}
	op, found := domain.FindChildOperation(record, cmd.OperationID, "")
	if !found || cmd.OperationID == "" {
		return result, fmt.Errorf("child operation not found")
	}
	result.ChildURL = op.CanonicalURL
	if record.IssueURL != op.ParentURL {
		return result, fmt.Errorf("child operation parent changed")
	}
	actor := cmd.Actor
	if record.Execution != nil {
		ancestry, err := observe()
		if err != nil {
			return result, err
		}
		actor.NativeProcessAncestry = ancestry
	}
	// Preview authorizes the reader but never records a replacement or receipt.
	if err := s.Intents.Authority.Authorize(ctx, record, actor); err != nil {
		return result, err
	}
	provider, err := s.Resolve(op.Provider)
	if err != nil {
		return result, err
	}
	childURL := op.CanonicalURL
	if childURL == "" {
		candidates, err := provider.FindChildCreateCandidates(ctx, port.IssueProviderFindIssueCreateCandidatesRequest{Repo: record.Repo, ProjectAuthority: op.ProjectAuthority, Marker: op.Marker})
		if err != nil {
			return result, err
		}
		result.CandidateCount = len(candidates.Candidates)
		if err := domain.ValidateIssueReconcileSearch(candidates.Truncated, len(candidates.Candidates)); err != nil {
			return result, err
		}
		candidate := candidates.Candidates[0]
		if err := validateChildSnapshot(op, port.ChildSnapshot{URL: candidate.URL, Title: candidate.Title, Body: candidate.Body}, false); err != nil {
			return result, err
		}
		childURL = candidate.URL
	} else {
		result.CandidateCount = 1
	}
	result.ChildURL = childURL
	failure := func(cause error, status string) error {
		if !cmd.Confirm {
			return cause
		}
		return errors.Join(cause, s.Intents.RecoveryFailure(context.Background(), record, op, childURL, status, remote.IssueCreateFailure(cause), actor))
	}
	snapshot, err := provider.ReadChild(ctx, record.Repo, op.ParentURL, childURL)
	if err != nil {
		return result, failure(err, model.IssueCreateIntentVerificationFailed)
	}
	if err := validateChildSnapshot(op, snapshot, true); err != nil {
		return result, failure(err, model.IssueCreateIntentVerificationFailed)
	}
	result.HierarchyVerified = snapshot.HierarchyVerified
	if !cmd.Confirm {
		result.OK = true
		result.WouldAdopt = true
		return result, nil
	}
	// Re-read before the only remote write: observations cannot grant a changed holder authority.
	current, err := s.Records.Read(ctx, cmd.ID)
	if err != nil {
		return result, err
	}
	if err := s.Intents.Authority.Authorize(ctx, current, actor); err != nil {
		return result, err
	}
	if !sameChildRecoveryAuthority(record, current) {
		return result, fmt.Errorf("child recovery authority changed")
	}
	if !snapshot.HierarchyVerified {
		if err := provider.AttachChild(ctx, record.Repo, op.ParentURL, childURL); err != nil {
			return result, failure(err, model.IssueCreateIntentVerificationFailed)
		}
		snapshot, err = provider.ReadChild(ctx, record.Repo, op.ParentURL, childURL)
		if err != nil {
			return result, failure(err, model.IssueCreateIntentVerificationFailed)
		}
		if err := validateChildSnapshot(op, snapshot, true); err != nil {
			return result, failure(err, model.IssueCreateIntentVerificationFailed)
		}
		if !snapshot.HierarchyVerified {
			return result, failure(fmt.Errorf("child hierarchy verification failed"), model.IssueCreateIntentVerificationFailed)
		}
	}
	if err := s.Intents.Complete(ctx, record, op, childURL, actor, true); err != nil {
		return result, failure(err, model.IssueCreateIntentReceiptFailed)
	}
	result.OK, result.HierarchyVerified, result.RecoveryCommand = true, true, ""
	return result, nil
}

func validateChildSnapshot(op model.ChildCreateOperation, snapshot port.ChildSnapshot, metadata bool) error {
	if snapshot.URL == op.ParentURL {
		return fmt.Errorf("child cannot be its own parent")
	}
	if err := remote.ValidateChildMatchesParent(op.ParentURL, snapshot.URL); err != nil {
		return err
	}
	if op.CanonicalURL != "" && op.CanonicalURL != snapshot.URL {
		return fmt.Errorf("child canonical URL mismatch")
	}
	if strings.TrimSpace(snapshot.Title) != op.Title || !strings.Contains(snapshot.Body, op.Marker) || fmt.Sprintf("%x", sha256.Sum256([]byte(strings.TrimSpace(snapshot.Body)))) != op.BodySHA256 {
		return fmt.Errorf("child marker/title/body mismatch")
	}
	if metadata {
		if !snapshot.TypeVerified {
			return fmt.Errorf("child type verification failed")
		}
		for _, pair := range [][2][]string{{op.Labels, snapshot.Labels}, {op.Assignees, snapshot.Assignees}} {
			if !containsChildValues(pair[1], pair[0]) {
				return fmt.Errorf("child metadata verification failed")
			}
		}
	}
	return nil
}
func containsChildValues(actual, want []string) bool {
	for _, w := range want {
		found := false
		for _, a := range actual {
			if strings.EqualFold(a, w) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func sameChildRecoveryAuthority(before, after model.IssueOpsRecord) bool {
	if before.IssueURL != after.IssueURL || before.Repo != after.Repo || before.Branch != after.Branch {
		return false
	}
	if before.Execution == nil {
		return after.Execution == nil
	}
	return after.Execution != nil && before.Execution.Lease.Generation == after.Execution.Lease.Generation && before.Execution.Workspace.Root == after.Execution.Workspace.Root
}
