package issueops

import (
	"context"
	"time"

	branchapp "issueops/internal/application/issueopsbranch"
	issueopscontract "issueops/internal/contract/issueops"
	linkedbranch "issueops/internal/domain/issueopslinkedbranch"
)

type AwaitBranchLinkDeps struct {
	Git                   func(ctx context.Context, dir string, args ...string) (int, string)
	ObserveLinkedBranches func(ctx context.Context, issueURL string) (linkedbranch.Observation, error)
	// Sleep과 Now는 테스트가 실제 시간을 기다리지 않게 하는 주입점이다.
	Sleep func(ctx context.Context, d time.Duration) error
	Now   func() time.Time
}

func AwaitBranchLink(ctx context.Context, root string, req issueopscontract.AwaitBranchLinkRequest, deps AwaitBranchLinkDeps) (issueopscontract.AwaitBranchLinkResult, error) {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Sleep == nil {
		deps.Sleep = SleepWithContext
	}
	return (branchapp.LinkAwaiter{
		Load:                  func(id string) (issueopscontract.IssueOpsRecord, error) { return ReadIssueOps(root, id) },
		RemoteRef:             (LinkedBranchRemoteRef{RunGit: deps.Git}).Observe,
		ObserveLinkedBranches: deps.ObserveLinkedBranches, Sleep: deps.Sleep, Now: deps.Now,
	}).Await(ctx, req)
}
