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
	// Settled proves that a pending child create whose caller died created
	// nothing and can no longer create anything (nil: never release).
	Settled func(context.Context, model.IssueOpsRecord, model.ChildCreateOperation) (bool, error)
}

// childRecoveryRun은 Reconcile 한 번 동안 고정되는 기록·작업·행위자·자식 URL이다.
type childRecoveryRun struct {
	record   model.IssueOpsRecord
	op       model.ChildCreateOperation
	actor    model.IssueOpsActor
	childURL string
	confirm  bool
}

func (s ChildReconciler) Reconcile(ctx context.Context, cmd ChildReconcileCommand, observe AncestryObserver) (model.ChildReconcileResult, error) {
	result := model.ChildReconcileResult{OperationID: cmd.OperationID, RecoveryCommand: ChildRecoveryCommand(cmd.ID, cmd.OperationID, cmd.Actor)}
	run, err := s.beginChildRecovery(ctx, cmd, observe, &result)
	if err != nil {
		return result, err
	}
	provider, err := s.Resolve(run.op.Provider)
	if err != nil {
		return result, err
	}
	run.childURL, err = s.locateRecoveryChild(ctx, provider, run, &result)
	if err != nil {
		// QA (killed create-child): a pending operation with no candidate
		// had no exit but abandon; release it once settled.
		if released, ok, releaseErr := s.releaseCrashedChild(ctx, cmd, run, result, err); ok {
			return released, releaseErr
		}
		return result, err
	}
	record, op, childURL := run.record, run.op, run.childURL
	result.ChildURL = childURL
	snapshot, err := provider.ReadChild(ctx, record.Repo, op.ParentURL, childURL)
	if err != nil {
		return result, s.recoveryFailure(ctx, run, err, model.IssueCreateIntentVerificationFailed)
	}
	if err := validateChildSnapshot(op, snapshot, true); err != nil {
		return result, s.recoveryFailure(ctx, run, err, model.IssueCreateIntentVerificationFailed)
	}
	result.HierarchyVerified = snapshot.HierarchyVerified
	if !cmd.Confirm {
		result.OK = true
		result.WouldAdopt = true
		return result, nil
	}
	if err := s.reauthorizeChildRecovery(ctx, cmd.ID, run); err != nil {
		return result, err
	}
	if !snapshot.HierarchyVerified {
		if err := s.attachRecoveredChild(ctx, provider, run); err != nil {
			return result, err
		}
	}
	if err := s.Intents.Complete(ctx, record, op, childURL, run.actor, true); err != nil {
		return result, s.recoveryFailure(ctx, run, err, model.IssueCreateIntentReceiptFailed)
	}
	result.OK, result.HierarchyVerified, result.RecoveryCommand = true, true, ""
	return result, nil
}

// beginChildRecovery는 기록과 작업을 읽어 부모 일치를 확인하고 미리보기 권한을 검증한다.
func (s ChildReconciler) beginChildRecovery(ctx context.Context, cmd ChildReconcileCommand, observe AncestryObserver, result *model.ChildReconcileResult) (childRecoveryRun, error) {
	run := childRecoveryRun{confirm: cmd.Confirm}
	record, err := s.Records.Read(ctx, cmd.ID)
	if err != nil {
		return run, err
	}
	run.record = record
	op, found := domain.FindChildOperation(record, cmd.OperationID, "")
	if !found || cmd.OperationID == "" {
		return run, fmt.Errorf("child operation not found")
	}
	run.op = op
	result.ChildURL = op.CanonicalURL
	if record.IssueURL != op.ParentURL {
		return run, fmt.Errorf("child operation parent changed")
	}
	run.actor = cmd.Actor
	if record.Execution != nil {
		ancestry, err := observe()
		if err != nil {
			return run, err
		}
		run.actor.NativeProcessAncestry = ancestry
	}
	// Preview authorizes the reader but never records a replacement or receipt.
	return run, s.Intents.Authority.Authorize(ctx, record, run.actor)
}

// releaseCrashedChild releases a pending child create that found no candidate
// when the settlement proves nothing was or will be created. ok is false when
// the release does not apply, leaving cause in place; an unsettled crash
// returns cause with the wait named.
func (s ChildReconciler) releaseCrashedChild(ctx context.Context, cmd ChildReconcileCommand, run childRecoveryRun, result model.ChildReconcileResult, cause error) (model.ChildReconcileResult, bool, error) {
	op := run.op
	if s.Settled == nil || result.CandidateCount != 0 || op.CanonicalURL != "" || op.Status != model.IssueCreateIntentPending {
		return result, false, nil
	}
	settled, err := s.Settled(ctx, run.record, op)
	if err != nil || !settled {
		// QA: an agent retried within the quiet period and read the bare
		// "exactly one is required" as a dead end; name the wait.
		hint := "not settled yet, wait and re-run this reconcile: a pending child create whose caller died is released once it is 2 minutes old (started " + op.UpdatedAt + "), no create for it is running, and the provider lists no child with its marker"
		if err != nil {
			hint += " (settlement: " + err.Error() + ")"
		}
		// The confirm command is for after the wait; the hint says when.
		result.RecoveryCommand = ""
		return result, true, fmt.Errorf("%w: %s", cause, hint)
	}
	if !cmd.Confirm {
		result.OK, result.WouldRelease = true, true
		return result, true, nil
	}
	if err := s.reauthorizeChildRecovery(ctx, cmd.ID, run); err != nil {
		return result, true, err
	}
	if err := s.Intents.ReleaseCrashed(ctx, run.record, op, run.actor); err != nil {
		return result, true, err
	}
	result.OK, result.Released, result.RecoveryCommand = true, true, ""
	return result, true, nil
}

// locateRecoveryChild는 작업에 기록된 자식 URL을 쓰거나, 없으면 marker 후보 검색으로 찾는다.
func (s ChildReconciler) locateRecoveryChild(ctx context.Context, provider port.IssueProviderChildCreateRecovery, run childRecoveryRun, result *model.ChildReconcileResult) (string, error) {
	op := run.op
	if op.CanonicalURL != "" {
		result.CandidateCount = 1
		return op.CanonicalURL, nil
	}
	candidates, err := provider.FindChildCreateCandidates(ctx, port.IssueProviderFindIssueCreateCandidatesRequest{Repo: run.record.Repo, ProjectAuthority: op.ProjectAuthority, Marker: op.Marker})
	if err != nil {
		return "", err
	}
	result.CandidateCount = len(candidates.Candidates)
	if err := domain.ValidateIssueReconcileSearch(candidates.Truncated, len(candidates.Candidates)); err != nil {
		return "", err
	}
	candidate := candidates.Candidates[0]
	if err := validateChildSnapshot(op, port.ChildSnapshot{URL: candidate.URL, Title: candidate.Title, Body: candidate.Body}, false); err != nil {
		return "", err
	}
	return candidate.URL, nil
}

// recoveryFailure는 확정 모드에서만 복구 실패를 durable 저장소에 남기고 오류를 합친다.
func (s ChildReconciler) recoveryFailure(ctx context.Context, run childRecoveryRun, cause error, status string) error {
	if !run.confirm {
		return cause
	}
	return errors.Join(cause, s.Intents.RecoveryFailure(context.WithoutCancel(ctx), run.record, run.op, run.childURL, status, remote.IssueCreateFailure(cause), run.actor))
}

// reauthorizeChildRecovery는 유일한 원격 쓰기 전에 기록을 다시 읽는다: 관찰만으로는 바뀐 holder에게 권한을 줄 수 없다.
func (s ChildReconciler) reauthorizeChildRecovery(ctx context.Context, id string, run childRecoveryRun) error {
	current, err := s.Records.Read(ctx, id)
	if err != nil {
		return err
	}
	if err := s.Intents.Authority.Authorize(ctx, current, run.actor); err != nil {
		return err
	}
	if !sameChildRecoveryAuthority(run.record, current) {
		return fmt.Errorf("child recovery authority changed")
	}
	return nil
}

// attachRecoveredChild는 계층이 빠진 자식을 부모에 연결하고 다시 읽어 계층을 검증한다.
func (s ChildReconciler) attachRecoveredChild(ctx context.Context, provider port.IssueProviderChildCreateRecovery, run childRecoveryRun) error {
	record, op, childURL := run.record, run.op, run.childURL
	if err := provider.AttachChild(ctx, record.Repo, op.ParentURL, childURL); err != nil {
		return s.recoveryFailure(ctx, run, err, model.IssueCreateIntentVerificationFailed)
	}
	snapshot, err := provider.ReadChild(ctx, record.Repo, op.ParentURL, childURL)
	if err != nil {
		return s.recoveryFailure(ctx, run, err, model.IssueCreateIntentVerificationFailed)
	}
	if err := validateChildSnapshot(op, snapshot, true); err != nil {
		return s.recoveryFailure(ctx, run, err, model.IssueCreateIntentVerificationFailed)
	}
	if !snapshot.HierarchyVerified {
		return s.recoveryFailure(ctx, run, fmt.Errorf("child hierarchy verification failed"), model.IssueCreateIntentVerificationFailed)
	}
	return nil
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
