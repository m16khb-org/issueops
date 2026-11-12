package issueopsapp

import (
	"context"
	"time"

	issueopsviewinbound "issueops/internal/adapter/inbound/issueopsview"
	issueopscore "issueops/internal/adapter/issueops"
	preflightadapter "issueops/internal/adapter/preflight"
	issueopsviewapplication "issueops/internal/application/issueopsview"
	issueopsviewcontract "issueops/internal/contract/issueopsview"
)

// issueOpsViewHandler는 `issueops view`의 읽기 포트를 composition root에서 묶는다.
// live record는 store를 만들거나 고치지 않는 ReadIssueOpsExisting으로,
// record가 지워진 cycle은 cleanup이나 prune이 남긴 보존 행을 ReadCycleOutcome으로 읽는다.
// graph 조망의 stale 표시는 `issueops next`와 같은 핸들러(next)가 내는 missing에서
// 오고, worktree 존재는 next가 쓰는 것과 같은 git 관측으로 본다. 둘 다 읽기 전용이다.
func issueOpsViewHandler(next func(string, string, string) (issueopsviewcontract.NextResult, error)) func(string, string, string, string) (issueopsviewcontract.Result, error) {
	return issueopsviewinbound.NewViewHandler(issueopsviewapplication.NewService(issueopsviewapplication.Ports{
		ReadRecord: func(ctx context.Context, stateRoot, id string) (issueopsviewcontract.Record, error) {
			return issueopscore.ReadIssueOpsExisting(stateRoot, id)
		},
		RetainedOutcome: func(ctx context.Context, stateRoot, id string) (issueopsviewcontract.CycleOutcome, bool, error) {
			return issueopscore.ReadCycleOutcome(stateRoot, id)
		},
		Next: func(ctx context.Context, stateRoot, cwd, id string) (issueopsviewcontract.NextResult, error) {
			return next(stateRoot, cwd, id)
		},
		// 존재만 본다: worktreeState가 존재로 치는 조건(빈 root가 아니고
		// show-toplevel이 값을 낸다)과 같고, branch와 HEAD는 읽지 않는다.
		WorktreePresent: func(ctx context.Context, root string) bool {
			return newNextGitObservation(preflightadapter.GitCmd).repository(root)
		},
		Now: time.Now,
	}))
}
