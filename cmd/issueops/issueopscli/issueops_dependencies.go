package issueopscli

import (
	"context"
	reviewcontract "issueops/internal/contract/issueopsreview"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsartifactcontract "issueops/internal/contract/issueopsartifact"
	issueopsdecisioncontract "issueops/internal/contract/issueopsdecision"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	issueopsretentioncontract "issueops/internal/contract/issueopsretention"
	issueopsroutingcontract "issueops/internal/contract/issueopsrouting"
	issueopsstatuscontract "issueops/internal/contract/issueopsstatus"
)

type IssueOpsCLIDeps struct {
	AcceptIssueOpsChildWithActor                func(stateRoot, parentID, childID string, evidence []string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildValidationResult, error)
	AddIssueOpsDecisionWithActor                func(stateRoot, id string, req issueopsdecisioncontract.Request, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	DropIssueOpsChildWithActor                  func(stateRoot, parentID, childID, reason string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildValidationResult, error)
	IssueOpsChildStatusWithActor                func(stateRoot, parentID string, repair bool, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildStatusResult, error)
	IssueOpsPRReadiness                         func(record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness
	IssueOpsNext                                func(stateRoot, cwd, id string) (issueopsnextcontract.Result, error)
	IssueOpsStateRoot                           func() string
	IssueOpsStatus                              func(stateRoot, id string) (issueopsstatuscontract.Record, error)
	LinkIssueOpsChildWithActor                  func(stateRoot, id, childURL, title string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	LinkIssueOpsIssueWithActor                  func(stateRoot, id, issueURL string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	LinkIssueOpsPlanWithActor                   func(stateRoot, id, planPath string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	LinkIssueOpsRelatedWithActor                func(stateRoot, id, linkType, relatedURL, title string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	LinkIssueOpsWorktreeWithActor               func(stateRoot, id, worktreePath string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	ListIssueOpsCycles                          func(stateRoot, repo string) (issueopsinventorycontract.ListResult, error)
	IssueOpsReviewMetrics                       func(stateRoot, id, repo string) (issueopscontract.IssueOpsReviewMetricsResult, error)
	ObserveNativeProcessAncestry                func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
	PrepareIssueOpsBranchWithActor              func(stateRoot, id string, req issueopscontract.IssueOpsBranchPrepareRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RetargetIssueOpsBranchWithActor             func(stateRoot, id string, req issueopscontract.IssueOpsBranchRetargetRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	AwaitIssueOpsBranchLink                     func(ctx context.Context, stateRoot string, req issueopscontract.AwaitBranchLinkRequest) (issueopscontract.AwaitBranchLinkResult, error)
	PruneIssueOps                               func(stateRoot string, maxAge time.Duration, confirm bool) (issueopsretentioncontract.Result, error)
	ReadIssueOps                                func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsAISlopCleanEvidenceWithActor  func(stateRoot, id string, categories, verification []string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsCompatibilityReviewWithActor  func(stateRoot, id string, req reviewcontract.CompatibilityReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsDesignReviewWithActor         func(stateRoot, id string, req reviewcontract.DesignReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsDevilsAdvocateReviewWithActor func(stateRoot, id string, req reviewcontract.DevilsAdvocateReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsDomainReviewWithActor         func(stateRoot, id string, req issueopscontract.IssueOpsDomainReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsImplementationReviewWithActor func(stateRoot, id string, req issueopscontract.IssueOpsImplementationReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsProjectDocsReviewWithActor    func(stateRoot, id string, req issueopscontract.IssueOpsProjectDocsReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsSchemaEvidenceWithActor       func(stateRoot, id string, req issueopscontract.IssueOpsSchemaEvidenceRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsIntentWithActor               func(stateRoot, id string, req issueopscontract.IssueOpsIntentRecordRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsPlanPrepWithActor             func(stateRoot, id string, req issueopscontract.IssueOpsPlanPrepRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsRoutingWithActor              func(stateRoot, id, phase, skill string, actor issueopscontract.IssueOpsActor) (issueopsroutingcontract.Record, error)
	RegressIssueOpsForReplanWithActor           func(stateRoot, id, reason string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RejectIssueOpsChildWithActor                func(stateRoot, parentID, childID, reason string, evidence []string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildValidationResult, error)
	ResolveIssueOpsFeedbackWithActor            func(stateRoot, id string, index int, resolution string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	ScoreLiveRoutingFidelity                    func(stateRoot, id string, expected []issueopsroutingcontract.Expected) (issueopsroutingcontract.Result, int, error)
	StageIssueOpsArtifact                       func(stateRoot, id, name string, content []byte) (issueopsartifactcontract.Record, error)
	StagedIssueOpsArtifactNames                 func(stateRoot, id string) ([]string, error)
	StartIssueOps                               func(stateRoot string, req issueopscontract.IssueOpsStartRequest) (issueopscontract.IssueOpsRecord, error)
	StartIssueOpsChildWithActor                 func(stateRoot string, req issueopscontract.IssueOpsChildStartRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildStartResult, error)
	UnstageIssueOpsArtifact                     func(stateRoot, id, name string) (issueopsartifactcontract.Record, error)
}
