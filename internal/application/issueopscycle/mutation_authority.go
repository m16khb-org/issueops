package issueopscycle

import (
	"context"
	"fmt"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	authorization "issueops/internal/domain/issueopsauthorization"
	authorityport "issueops/internal/port/authority"
)

// MutationAuthority 타입은 실행 기록 변경 권한을 검사한다. 실행 기록 전체 검증과 lease holder 확인을 함께 수행한다.
type MutationAuthority struct {
	pathsMatch func(string, string) bool
	verifier   authorityport.ActorVerifier
}

// NewMutationAuthority 함수는 경로 비교 함수와 행위자 검증기로 MutationAuthority를 만든다.
func NewMutationAuthority(pathsMatch func(string, string) bool, verifier authorityport.ActorVerifier) MutationAuthority {
	return MutationAuthority{pathsMatch: pathsMatch, verifier: verifier}
}

// Authorize 메서드는 actor가 기록의 write lease holder로서 변경할 권한이 있는지 검사한다.
func (a MutationAuthority) Authorize(ctx context.Context, record model.Record, actor model.Actor) error {
	return a.Validate(ctx, record, &actor)
}

// Validate 메서드는 기록의 실행 상태가 유효한지 확인한 뒤 actor를 holder로 검증한다. 검증 과정에서 actor의 정보를 채울 수 있다.
func (a MutationAuthority) Validate(ctx context.Context, record model.Record, actor *model.Actor) error {
	if record.Execution != nil {
		if err := domain.ValidateExecution(*record.Execution); err != nil {
			return fmt.Errorf("invalid IssueOps execution v1 record: %w", err)
		}
	}
	return AuthorizeHolder(ctx, record, actor, a.pathsMatch, a.verifier)
}

// AuthorizeHolder preserves the holder-only contract used by decision and routing.
// Callers that require complete execution validation use MutationAuthority.
func AuthorizeHolder(ctx context.Context, record model.Record, actor *model.Actor, pathsMatch func(string, string) bool, verifier authorityport.ActorVerifier) error {
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
	root := record.Execution.Workspace.Root
	return authorization.ValidateCWD(pathsMatch != nil && pathsMatch(actor.CWD, root), actor.CWD, root)
}

// verifyHolderCaller builds the native verification input from the caller's
// observed ancestry and the holder's recorded session receipt. A bound
// capability ignores that ancestry and proves the caller by its grant.
func verifyHolderCaller(ctx context.Context, record model.Record, actor *model.Actor, verifier authorityport.ActorVerifier) (*model.VerifiedActor, error) {
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
