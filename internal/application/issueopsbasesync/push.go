package issueopsbasesync

import (
	"context"
	"fmt"
	"strings"

	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// PushRequest carries what PushAndRecord needs to push Branch from Root and
// record the sync-base event: lifecycle ID, mode, base branch and OID, the
// merge commit, the number of conflicted files, and the acting identity and
// timestamp of the event.
type PushRequest struct {
	ID            string
	Root          string
	Branch        string
	Mode          string
	BaseBranch    string
	BaseOID       string
	MergeCommit   string
	ConflictFiles int
	Actor         string
	At            string
}

// PushResult reports whether the branch was pushed, whether the push must be
// retried, and which step (push or record_event) failed.
type PushResult struct {
	Pushed            bool
	PushRetryRequired bool
	FailedStep        string
}

// PushAndRecord keeps the durable success event behind the external push.
func PushAndRecord(ctx context.Context, request PushRequest, effects port.BaseSyncPushEffects) (PushResult, error) {
	if effects == nil {
		return PushResult{}, fmt.Errorf("base sync push effects are required")
	}
	refspec := "refs/heads/" + request.Branch + ":refs/heads/" + request.Branch
	if code, out := effects.Push(ctx, request.Root, refspec); code != 0 {
		return PushResult{PushRetryRequired: true, FailedStep: "push"}, fmt.Errorf("git push origin %s: %s", refspec, strings.TrimSpace(out))
	}
	event := issueops.ExecutionSyncBaseEvent{
		Mode: request.Mode, BaseBranch: request.BaseBranch, BaseOID: request.BaseOID,
		MergeCommit: request.MergeCommit, ConflictFiles: request.ConflictFiles,
		Actor: request.Actor, At: request.At,
	}
	if err := effects.AppendEvent(ctx, request.ID, event); err != nil {
		return PushResult{Pushed: true, FailedStep: "record_event"}, err
	}
	return PushResult{Pushed: true}, nil
}
