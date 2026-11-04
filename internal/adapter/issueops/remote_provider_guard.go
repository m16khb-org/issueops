package issueops

import (
	"context"
	"fmt"
	executionissue "issueops/internal/contract/executionissue"

	"issueops/internal/port"
)

// Provider 호출 경계에서 필요한 capability를 확인한다.
// Lifecycle 판단과 실행 순서는 application이 소유한다.

func CreateRemoteIssueContext(ctx context.Context, req port.IssueProviderCreateIssueRequest, prov port.IssueProvider) (port.IssueProviderCreateIssueResult, error) {
	if prov == nil {
		return port.IssueProviderCreateIssueResult{OK: false}, fmt.Errorf("no issue provider configured")
	}
	return prov.CreateIssue(ctx, req)
}

// ReadRemoteIssueSnapshot은 provider가 snapshot 읽기를 지원할 때만 이슈 본문을
// 읽는다. 모든 provider가 이 능력을 갖추지는 않으므로 타입 단언으로 확인한다.
func ReadRemoteIssueSnapshot(ctx context.Context, prov port.IssueProvider, req executionissue.SnapshotRequest) (executionissue.Snapshot, error) {
	reader, ok := prov.(port.ExecutionIssueSnapshotReader)
	if !ok {
		return executionissue.Snapshot{}, fmt.Errorf("issue provider does not support issue snapshot reads")
	}
	return reader.ReadIssueSnapshot(ctx, req)
}

// CreateRemotePullRequestViaProviderContext는 provider가 구성되어 있을 때만
// PR을 만든다. 이 함수는 provider 호출 직전의 capability 가드만 담당하며
// lifecycle 상태와 actor 검증은 application이 소유한다.
func CreateRemotePullRequestViaProviderContext(ctx context.Context, req port.IssueProviderCreatePullRequestRequest, prov port.IssueProvider) (executionissue.IssueProviderCreatePullRequestResult, error) {
	if prov == nil {
		return executionissue.IssueProviderCreatePullRequestResult{OK: false}, fmt.Errorf("no issue provider configured")
	}
	return prov.CreatePullRequest(ctx, req)
}

// ReconcileRemotePullRequestViaProviderContext는 provider가 remote create 조정을
// 지원할 때만 조정을 수행한다.
func ReconcileRemotePullRequestViaProviderContext(ctx context.Context, req port.IssueProviderReconcilePullRequestRequest, prov port.IssueProvider) (port.IssueProviderReconcilePullRequestResult, error) {
	reconciler, ok := prov.(port.IssueProviderRemoteCreateReconciler)
	if !ok {
		return port.IssueProviderReconcilePullRequestResult{}, fmt.Errorf("issue provider does not support remote create reconciliation")
	}
	return reconciler.ReconcilePullRequest(ctx, req)
}
