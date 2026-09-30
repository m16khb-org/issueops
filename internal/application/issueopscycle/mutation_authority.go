package issueopscycle

import (
	"context"
	"fmt"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	authorization "issueops/internal/domain/issueopsauthorization"
)

type MutationAuthority struct{ pathsMatch func(string, string) bool }

func NewMutationAuthority(pathsMatch func(string, string) bool) MutationAuthority {
	return MutationAuthority{pathsMatch: pathsMatch}
}

func (a MutationAuthority) Authorize(_ context.Context, record model.IssueOpsRecord, actor model.IssueOpsActor) error {
	return a.Validate(record, &actor)
}

func (a MutationAuthority) Validate(record model.IssueOpsRecord, actor *model.IssueOpsActor) error {
	if record.Execution != nil {
		if err := domain.ValidateExecution(*record.Execution); err != nil {
			return fmt.Errorf("invalid IssueOps execution v1 record: %w", err)
		}
	}
	return AuthorizeHolder(record, actor, a.pathsMatch)
}

// AuthorizeHolder preserves the holder-only contract used by decision and routing.
// Callers that require complete execution validation use MutationAuthority.
func AuthorizeHolder(record model.IssueOpsRecord, actor *model.IssueOpsActor, pathsMatch func(string, string) bool) error {
	needsPath, err := authorization.ValidateHolder(record, actor)
	if err != nil {
		return err
	}
	if !needsPath {
		return nil
	}
	return authorization.ValidateCWD(pathsMatch != nil && pathsMatch(actor.CWD, record.Execution.Workspace.Root))
}
