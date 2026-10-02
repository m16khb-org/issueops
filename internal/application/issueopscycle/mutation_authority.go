package issueopscycle

import (
	"context"
	"fmt"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	authorization "issueops/internal/domain/issueopsauthorization"
	authorityport "issueops/internal/port/authority"
)

type MutationAuthority struct {
	pathsMatch func(string, string) bool
	verifier   authorityport.ActorVerifier
}

func NewMutationAuthority(pathsMatch func(string, string) bool, verifier authorityport.ActorVerifier) MutationAuthority {
	return MutationAuthority{pathsMatch: pathsMatch, verifier: verifier}
}

func (a MutationAuthority) Authorize(ctx context.Context, record model.IssueOpsRecord, actor model.IssueOpsActor) error {
	return a.Validate(ctx, record, &actor)
}

func (a MutationAuthority) Validate(ctx context.Context, record model.IssueOpsRecord, actor *model.IssueOpsActor) error {
	if record.Execution != nil {
		if err := domain.ValidateExecution(*record.Execution); err != nil {
			return fmt.Errorf("invalid IssueOps execution v1 record: %w", err)
		}
	}
	return AuthorizeHolder(ctx, record, actor, a.pathsMatch, a.verifier)
}

// AuthorizeHolder preserves the holder-only contract used by decision and routing.
// Callers that require complete execution validation use MutationAuthority.
func AuthorizeHolder(ctx context.Context, record model.IssueOpsRecord, actor *model.IssueOpsActor, pathsMatch func(string, string) bool, verifier authorityport.ActorVerifier) error {
	verified, err := verifyHolderCaller(ctx, record, actor, verifier)
	if err != nil {
		return err
	}
	needsPath, err := authorization.ValidateHolder(record, actor, verified)
	if err != nil {
		return err
	}
	if !needsPath {
		return nil
	}
	return authorization.ValidateCWD(pathsMatch != nil && pathsMatch(actor.CWD, record.Execution.Workspace.Root))
}

// verifyHolderCaller builds the native verification input from the caller's
// observed ancestry and the holder's recorded session receipt. A bound
// capability ignores that ancestry and proves the caller by its grant.
func verifyHolderCaller(ctx context.Context, record model.IssueOpsRecord, actor *model.IssueOpsActor, verifier authorityport.ActorVerifier) (*model.VerifiedActor, error) {
	if record.Execution == nil || actor == nil || verifier == nil {
		return nil, nil
	}
	holder := record.Execution.Lease.Holder
	if record.Execution.Lease.Status != model.LeaseStatusActive || holder == nil || holder.SessionProcess == nil {
		return nil, nil
	}
	process := *holder.SessionProcess
	verified, err := verifier.Verify(ctx, model.NativeActor{
		Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID,
		SessionProcess: &process, ProcessAncestry: actor.NativeProcessAncestry,
	})
	if err != nil {
		return nil, fmt.Errorf("IssueOps execution mutation requires the current write lease holder: %w", err)
	}
	return &verified, nil
}
