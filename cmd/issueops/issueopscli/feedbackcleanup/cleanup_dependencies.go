package feedbackcleanup

import (
	"context"

	cleanupapp "issueops/internal/application/issueopscleanup"
	issueopscontract "issueops/internal/contract/issueops"
)

type Command struct {
	Operations CleanupDeps
	Invoke     func(Deps) cleanupapp.Invocation
}

type CleanupDeps struct {
	Status                                            func(context.Context, string, string, bool, Deps) (issueopscontract.CleanupStatus, error)
	AddIssueOpsFeedbackWithActor                      func(stateRoot, id, source, body, classification string, actor issueopscontract.Actor) (issueopscontract.Record, error)
	CloseIssueOpsChildren                             func(stateRoot, id string, req issueopscontract.CloseChildrenRequest, deps Deps) (issueopscontract.CloseChildrenResult, error)
	IssueOpsStateRoot                                 func() string
	MarkIssueOpsContractFeedbackIssueUpdatedWithActor func(stateRoot, id string, actor issueopscontract.Actor) (issueopscontract.Record, error)
	ObserveNativeProcessAncestry                      func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
}
