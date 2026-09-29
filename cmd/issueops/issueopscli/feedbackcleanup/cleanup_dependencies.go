package feedbackcleanup

import (
	"context"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type Command struct{ Operations CleanupDeps }

type CleanupDeps struct {
	Status                                            func(context.Context, string, string, bool, Deps) (issueopscontract.IssueOpsCleanupStatus, error)
	AddIssueOpsFeedbackWithActor                      func(stateRoot, id, source, body, classification string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	CleanupAbandon                                    func(ctx context.Context, stateRoot string, req issueopscontract.CleanupAbandonRequest, deps Deps) (issueopscontract.CleanupAbandonResult, error)
	CleanupFinish                                     func(ctx context.Context, stateRoot string, req issueopscontract.CleanupFinishRequest, deps Deps, prov port.IssueProvider) (issueopscontract.CleanupFinishResult, error)
	CleanupRemoteBranch                               func(ctx context.Context, stateRoot string, req issueopscontract.CleanupRemoteBranchRequest, deps Deps, prov port.IssueProvider) (issueopscontract.CleanupRemoteBranchResult, error)
	CleanupLinkedBranch                               func(ctx context.Context, stateRoot string, req issueopscontract.CleanupLinkedBranchRequest) (issueopscontract.CleanupLinkedBranchResult, error)
	CloseIssueOpsChildren                             func(stateRoot, id string, req issueopscontract.IssueOpsCloseChildrenRequest, deps Deps) (issueopscontract.IssueOpsCloseChildrenResult, error)
	IssueOpsStateRoot                                 func() string
	MarkIssueOpsContractFeedbackIssueUpdatedWithActor func(stateRoot, id string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	ObserveNativeProcessAncestry                      func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
	ReadIssueOps                                      func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error)
	ResolveRecordProvider                             func(issueopscontract.IssueOpsRecord) string
}
