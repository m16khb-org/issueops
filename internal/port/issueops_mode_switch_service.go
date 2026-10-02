package port

import (
	"context"
	model "issueops/internal/contract/issueops"
)

type ModeSwitchRecords interface {
	Load(string) (model.IssueOpsRecord, error)
	Save(context.Context, model.IssueOpsRecord) (model.IssueOpsRecord, error)
	WithinLock(context.Context, string, func(context.Context) error) error
}
type ModeSwitchWorkspace interface {
	Present(string) bool
	Clean(string) bool
	CommitCount(root, ref string) (string, bool)
	BranchOID(repo, branch string, remote bool) (string, bool)
	RemoveWorktree(context.Context, string, string) error
	RemoveBranch(context.Context, string, string) error
}
