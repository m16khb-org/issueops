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

// Deps 타입은 IssueOps 하위 명령이 쓰는 상태 저장소 연산을 함수 필드로 모은다.
// 테스트와 composition root가 구현을 바꿔 끼운다.
type Deps struct {
	AcceptIssueOpsChildWithActor                func(stateRoot, parentID, childID string, evidence []string, actor issueopscontract.Actor) (issueopscontract.ChildValidationResult, error)
	AddIssueOpsDecisionWithActor                func(stateRoot, id string, req issueopsdecisioncontract.Request, actor issueopscontract.Actor) (issueopscontract.Record, error)
	DropIssueOpsChildWithActor                  func(stateRoot, parentID, childID, reason string, actor issueopscontract.Actor) (issueopscontract.ChildValidationResult, error)
	IssueOpsChildStatusWithActor                func(stateRoot, parentID string, repair bool, actor issueopscontract.Actor) (issueopscontract.ChildStatusResult, error)
	IssueOpsPRReadiness                         func(record issueopscontract.Record) issueopscontract.Readiness
	IssueOpsNext                                func(stateRoot, cwd, id string) (issueopsnextcontract.Result, error)
	IssueOpsStateRoot                           func() string
	IssueOpsStatus                              func(stateRoot, id string) (issueopsstatuscontract.Record, error)
	LinkIssueOpsChildWithActor                  func(stateRoot, id, childURL, title string, actor issueopscontract.Actor) (issueopscontract.Record, error)
	LinkIssueOpsIssueWithActor                  func(stateRoot, id, issueURL string, actor issueopscontract.Actor) (issueopscontract.Record, error)
	LinkIssueOpsPlanWithActor                   func(stateRoot, id, planPath string, actor issueopscontract.Actor) (issueopscontract.Record, error)
	LinkIssueOpsRelatedWithActor                func(stateRoot, id, linkType, relatedURL, title string, actor issueopscontract.Actor) (issueopscontract.Record, error)
	LinkIssueOpsWorktreeWithActor               func(stateRoot, id, worktreePath string, actor issueopscontract.Actor) (issueopscontract.Record, error)
	ListIssueOpsCycles                          func(stateRoot, repo string) (issueopsinventorycontract.ListResult, error)
	IssueOpsReviewMetrics                       func(stateRoot, id, repo string) (issueopscontract.ReviewMetricsResult, error)
	ObserveNativeProcessAncestry                func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
	PrepareIssueOpsBranchWithActor              func(stateRoot, id string, req issueopscontract.BranchPrepareRequest, actor issueopscontract.Actor) (issueopscontract.Record, error)
	RetargetIssueOpsBranchWithActor             func(stateRoot, id string, req issueopscontract.BranchRetargetRequest, actor issueopscontract.Actor) (issueopscontract.Record, error)
	AwaitIssueOpsBranchLink                     func(ctx context.Context, stateRoot string, req issueopscontract.AwaitBranchLinkRequest) (issueopscontract.AwaitBranchLinkResult, error)
	PruneIssueOps                               func(stateRoot string, maxAge time.Duration, confirm bool) (issueopsretentioncontract.Result, error)
	ReadIssueOps                                func(stateRoot, id string) (issueopscontract.Record, error)
	RecordIssueOpsAISlopCleanEvidenceWithActor  func(stateRoot, id string, categories, verification []string, actor issueopscontract.Actor) (issueopscontract.Record, error)
	RecordIssueOpsCompatibilityReviewWithActor  func(stateRoot, id string, req reviewcontract.CompatibilityReviewRequest, actor issueopscontract.Actor) (issueopscontract.Record, error)
	RecordIssueOpsArchitectureReviewWithActor   func(stateRoot, id string, req reviewcontract.ArchitectureReviewRequest, actor issueopscontract.Actor) (issueopscontract.Record, error)
	RecordIssueOpsDevilsAdvocateReviewWithActor func(stateRoot, id string, req reviewcontract.DevilsAdvocateReviewRequest, actor issueopscontract.Actor) (issueopscontract.Record, error)
	RecordIssueOpsDomainReviewWithActor         func(stateRoot, id string, req issueopscontract.DomainReviewRequest, actor issueopscontract.Actor) (issueopscontract.Record, error)
	RecordIssueOpsImplementationReviewWithActor func(stateRoot, id string, req issueopscontract.ImplementationReviewRequest, actor issueopscontract.Actor) (issueopscontract.Record, error)
	RecordIssueOpsProjectDocsReviewWithActor    func(stateRoot, id string, req issueopscontract.ProjectDocsReviewRequest, actor issueopscontract.Actor) (issueopscontract.Record, error)
	RecordIssueOpsSchemaEvidenceWithActor       func(stateRoot, id string, req issueopscontract.SchemaEvidenceRequest, actor issueopscontract.Actor) (issueopscontract.Record, error)
	RecordIssueOpsIntentWithActor               func(stateRoot, id string, req issueopscontract.IntentRecordRequest, actor issueopscontract.Actor) (issueopscontract.Record, error)
	RecordIssueOpsPlanPrepWithActor             func(stateRoot, id string, req issueopscontract.PlanPrepRequest, actor issueopscontract.Actor) (issueopscontract.Record, error)
	RecordIssueOpsRoutingWithActor              func(stateRoot, id, phase, skill string, actor issueopscontract.Actor) (issueopsroutingcontract.Record, error)
	RegressIssueOpsForReplanWithActor           func(stateRoot, id, reason string, actor issueopscontract.Actor) (issueopscontract.Record, error)
	RejectIssueOpsChildWithActor                func(stateRoot, parentID, childID, reason string, evidence []string, actor issueopscontract.Actor) (issueopscontract.ChildValidationResult, error)
	ResolveIssueOpsFeedbackWithActor            func(stateRoot, id string, index int, resolution string, actor issueopscontract.Actor) (issueopscontract.Record, error)
	ScoreLiveRoutingFidelity                    func(stateRoot, id string, expected []issueopsroutingcontract.Expected) (issueopsroutingcontract.Result, int, error)
	StageIssueOpsArtifact                       func(stateRoot, id, name string, content []byte) (issueopsartifactcontract.Record, error)
	StagedIssueOpsArtifactNames                 func(stateRoot, id string) ([]string, error)
	StartIssueOps                               func(stateRoot string, req issueopscontract.StartRequest) (issueopscontract.Record, error)
	StartIssueOpsChildWithActor                 func(stateRoot string, req issueopscontract.ChildStartRequest, actor issueopscontract.Actor) (issueopscontract.ChildStartResult, error)
	UnstageIssueOpsArtifact                     func(stateRoot, id, name string) (issueopsartifactcontract.Record, error)
}
