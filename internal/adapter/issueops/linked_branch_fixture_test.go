package issueops

import (
	"context"
	app "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	linkedbranch "issueops/internal/domain/issueopslinkedbranch"
	"time"
)

type CleanupLinkedBranchDeps struct {
	Git func(ctx context.Context, dir string, args ...string) (int, string)
	// ObserveLinkedBranches는 이슈의 linked-branch 목록을 읽는다. TotalCount와
	// Nodes를 채워야 하며, 나머지 필드는 호출부가 record에서 채운다.
	ObserveLinkedBranches func(ctx context.Context, issueURL string) (linkedbranch.Observation, error)
	// DeleteLinkedBranch는 노드 id 하나만 지운다. 브랜치 이름을 받지 않는 것이
	// 의도다 — 이름으로 지우는 표면이 있으면 ref 있는 링크도 지울 수 있게 된다.
	DeleteLinkedBranch func(ctx context.Context, issueURL, nodeID string) error
}

func CleanupLinkedBranch(ctx context.Context, root string, req model.CleanupLinkedBranchRequest, deps CleanupLinkedBranchDeps) (model.CleanupLinkedBranchResult, error) {
	return (app.LinkedBranchCleaner{Records: CycleRecordStore{StateRoot: root}, RemoteRef: LinkedBranchRemoteRef{RunGit: deps.Git}.Observe, ObserveLinkedBranches: deps.ObserveLinkedBranches, DeleteLinkedBranch: deps.DeleteLinkedBranch, Now: time.Now}).Run(ctx, req)
}
