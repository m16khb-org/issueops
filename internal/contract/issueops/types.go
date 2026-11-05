// Package issueops defines the IssueOps cycle contract: the durable Record and
// the request/result DTOs, execution lease, cleanup, remote-artifact and handoff
// vocabulary that the domain, application and adapter layers exchange. It holds
// data shapes and their validation only and performs no I/O, so every layer can
// depend on it without depending on each other.
package issueops

import reviewcontract "issueops/internal/contract/issueopsreview"

// StartRequest asks to start (or resume) the cycle for Repo on Branch.
type StartRequest struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch,omitempty"`
	// New requests a fresh branchless lifecycle instead of deterministic resume.
	New bool `json:"new,omitempty"`
}

// FeedbackItem is one piece of intake feedback attached to a cycle, with the
// timestamps needed to detect that the source issue changed after it was read.
type FeedbackItem struct {
	Source         string `json:"source"`
	Body           string `json:"body"`
	Classification string `json:"classification,omitempty"`
	CreatedAt      string `json:"created_at"`
	IssueUpdatedAt string `json:"issue_updated_at,omitempty"`
	// Resolution records the outcome of the feedback item, distinct from the
	// intake Classification (e.g. valid-defect | question-answered | noise-dismissed).
	Resolution string `json:"resolution,omitempty"`
}

// IssueLink is a remote issue-tracker artifact linked to a cycle, including the
// close state the harness verified for it.
type IssueLink struct {
	Type            string `json:"type"`
	URL             string `json:"url"`
	Title           string `json:"title,omitempty"`
	Provider        string `json:"provider,omitempty"`
	CreatedAt       string `json:"created_at"`
	ClosedAt        string `json:"closed_at,omitempty"`
	CloseVerifiedAt string `json:"close_verified_at,omitempty"`
	CloseReason     string `json:"close_reason,omitempty"`
}

// BranchPrepareStep is one ordered provider instruction (tool call or command)
// for creating the issue's linked branch.
type BranchPrepareStep struct {
	Order         int            `json:"order"`
	Strategy      string         `json:"strategy"`
	Tool          string         `json:"tool,omitempty"`
	ToolArguments map[string]any `json:"tool_arguments,omitempty"`
	Command       []string       `json:"command,omitempty"`
	Description   string         `json:"description"`
}

// BranchPrepare is the sealed branch identity of a cycle: the issue, the linked
// branch, its base, and the provider-verified link state.
type BranchPrepare struct {
	Provider        string `json:"provider"`
	IssueURL        string `json:"issue_url"`
	Branch          string `json:"branch"`
	BaseBranch      string `json:"base_branch"`
	BaseSHA         string `json:"base_sha,omitempty"`
	ParentWorktree  string `json:"parent_worktree,omitempty"`
	RemoteBranchURL string `json:"remote_branch_url,omitempty"`
	// CodeProjectKey is the provider project that owns the branch and will own
	// the PR/MR. It is empty when the code lives in the issue's own project,
	// which keeps every same-project cycle on the historical path; it is sealed
	// only when the two differ, and artifact validation then binds to it.
	CodeProjectKey string `json:"code_project_key,omitempty"`
	LinkVerified   bool   `json:"link_verified"`
	// Steps are the provider instructions for creating the linked branch. The
	// `branch prepare` acknowledgement carries them once; the `status` view
	// leaves them out because they never change after prepare.
	Steps     []BranchPrepareStep `json:"steps,omitempty"`
	CreatedAt string              `json:"created_at"`
	// Retargets records every provider-observed base change after prepare, oldest
	// first. BaseBranch always equals the last entry's ToBase.
	Retargets []BranchRetarget `json:"retargets,omitempty"`
}

// BranchRetarget is one accepted base change. ToBase was read back from
// the remote artifact and observed on origin at ObservedAt; it is never asserted.
type BranchRetarget struct {
	FromBase    string `json:"from_base"`
	ToBase      string `json:"to_base"`
	Reason      string `json:"reason"`
	ArtifactURL string `json:"artifact_url"`
	ObservedAt  string `json:"observed_at"`
}

// BranchRetargetRequest asks to move the cycle's recorded base branch to
// BaseBranch for Reason; the new base must be read back from the remote artifact.
type BranchRetargetRequest struct {
	BaseBranch string
	Reason     string
}

// BranchPrepareRequest carries the caller-supplied fields that become a
// BranchPrepare when the branch identity is recorded.
type BranchPrepareRequest struct {
	Provider        string `json:"provider"`
	IssueURL        string `json:"issue_url"`
	Branch          string `json:"branch"`
	BaseBranch      string `json:"base_branch"`
	BaseSHA         string `json:"base_sha,omitempty"`
	ParentWorktree  string `json:"parent_worktree,omitempty"`
	RemoteBranchURL string `json:"remote_branch_url,omitempty"`
	CodeProjectKey  string `json:"code_project_key,omitempty"`
	LinkVerified    bool   `json:"link_verified,omitempty"`
}

// MaxIssueOpsBodySyncs bounds the sync baselines a record keeps. One entry per
// artifact is enough for staleness detection, and the cap stops a long-running
// cycle with many children from growing the record without limit.
const MaxIssueOpsBodySyncs = 16

// RemoteBodySync is the last body the harness wrote to one remote
// artifact. It is the baseline the next staleness check compares against, so a
// body edited outside the harness shows up as an outside edit instead of being
// silently overwritten.
type RemoteBodySync struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
	// FromSHA256 is the body that was replaced; ToSHA256 is what the provider
	// read back afterwards, never what the caller intended to write.
	FromSHA256 string `json:"from_sha256,omitempty"`
	ToSHA256   string `json:"to_sha256"`
	// Generation records the execution lease that authorized a PR/MR sync.
	Generation uint64 `json:"generation,omitempty"`
	SyncedAt   string `json:"synced_at"`
}

// RemoteArtifactVerification records what the provider reported for the cycle's
// PR/MR (labels, assignees, target branch) at VerifiedAt.
type RemoteArtifactVerification struct {
	Provider   string   `json:"provider"`
	Kind       string   `json:"kind"`
	URL        string   `json:"url"`
	Labels     []string `json:"labels"`
	Assignees  []string `json:"assignees"`
	VerifiedAt string   `json:"verified_at"`
	// TargetBranch is the PR/MR target branch, compared to BranchPrepare.BaseBranch
	// for the target_branch_match check.
	TargetBranch string `json:"target_branch,omitempty"`
}

// RemoteArtifactVerificationRequest carries the provider readback to be stored
// as a RemoteArtifactVerification.
type RemoteArtifactVerificationRequest struct {
	Provider     string
	Kind         string
	URL          string
	Labels       []string
	Assignees    []string
	TargetBranch string
}

// IntentContract is the interpreted form of the user's request: what counts as
// success, and the constraints, ambiguities and non-goals around it.
type IntentContract struct {
	RawRequest        string   `json:"raw_request"`
	InterpretedIntent string   `json:"interpreted_intent"`
	SuccessCriteria   []string `json:"success_criteria"`
	Constraints       []string `json:"constraints,omitempty"`
	Ambiguities       []string `json:"ambiguities,omitempty"`
	NonGoals          []string `json:"non_goals,omitempty"`
	IntentClass       string   `json:"intent_class,omitempty"`
	RecordedAt        string   `json:"recorded_at"`
}

// IntentRecordRequest carries the fields recorded as an IntentContract.
type IntentRecordRequest struct {
	RawRequest        string
	InterpretedIntent string
	SuccessCriteria   []string
	Constraints       []string
	Ambiguities       []string
	NonGoals          []string
	IntentClass       string
}

// Decision is one recorded design or scope decision with its rationale,
// alternatives and the artifacts it affects.
type Decision struct {
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	Kind               string   `json:"kind"`
	Rationale          string   `json:"rationale,omitempty"`
	Alternatives       []string `json:"alternatives,omitempty"`
	AffectedIssueLinks []string `json:"affected_issue_links,omitempty"`
	AffectedArtifacts  []string `json:"affected_artifacts,omitempty"`
	CreatedAt          string   `json:"created_at"`
}

// DecisionRecordRequest carries the fields recorded as a Decision.
type DecisionRecordRequest struct {
	Title              string
	Body               string
	Kind               string
	Rationale          string
	Alternatives       []string
	AffectedIssueLinks []string
	AffectedArtifacts  []string
}

// PlanPrepItem is the outcome of one plan-preparation step: done with Evidence,
// or waived with WaiveReason.
type PlanPrepItem struct {
	Status      string   `json:"status"`
	Evidence    []string `json:"evidence,omitempty"`
	WaiveReason string   `json:"waive_reason,omitempty"`
}

// PlanPrep records the four plan-preparation checks (prior decisions, related
// issues, web research, codebase survey) completed before planning.
type PlanPrep struct {
	PriorDecisions PlanPrepItem `json:"prior_decisions"`
	RelatedIssues  PlanPrepItem `json:"related_issues"`
	WebResearch    PlanPrepItem `json:"web_research"`
	CodebaseSurvey PlanPrepItem `json:"codebase_survey"`
	RecordedAt     string       `json:"recorded_at"`
}

// PlanPrepItemRequest carries the evidence or waiver for one plan-prep step.
type PlanPrepItemRequest struct {
	Evidence    []string
	WaiveReason string
}

// PlanPrepRequest carries the four plan-prep step inputs recorded as a PlanPrep.
type PlanPrepRequest struct {
	PriorDecisions PlanPrepItemRequest
	RelatedIssues  PlanPrepItemRequest
	WebResearch    PlanPrepItemRequest
	CodebaseSurvey PlanPrepItemRequest
}

// DevilsAdvocateReview aliases the plan-review verdict defined in the review
// contract. domain/issueops may import only this contract package, and its
// plan-review rendering names this type, so it stays visible here.
type DevilsAdvocateReview = reviewcontract.DevilsAdvocateReview

// DevilsAdvocateRound aliases one adversarial round of a DevilsAdvocateReview,
// re-exported here for the same import-boundary reason.
type DevilsAdvocateRound = reviewcontract.DevilsAdvocateRound

// DomainReview captures the grill-phase domain grilling outcome:
// terminology, current model fit, risks, and unresolved uncertainties. It is a
// net-new source-of-truth field; grilling produced no record state before.
type DomainReview struct {
	Terminology       []string `json:"terminology,omitempty"`
	ModelFit          string   `json:"model_fit,omitempty"`
	Risks             []string `json:"risks,omitempty"`
	OpenUncertainties []string `json:"open_uncertainties,omitempty"`
	ReviewedAt        string   `json:"reviewed_at"`
}

// DomainReviewRequest carries the fields recorded as a DomainReview.
type DomainReviewRequest struct {
	Terminology       []string
	ModelFit          string
	Risks             []string
	OpenUncertainties []string
}

// PhaseLedgerEntry records that a phase was entered and (optionally)
// completed, plus which artifacts satisfied it. It is an index over existing
// source-of-truth fields, not their replacement. The owning map's key is the
// authoritative phase identity; Phase is a self-describing copy that must equal
// its key.
type PhaseLedgerEntry struct {
	Phase       Phase    `json:"phase"`
	EnteredAt   string   `json:"entered_at,omitempty"`
	CompletedAt string   `json:"completed_at,omitempty"`
	Artifacts   []string `json:"artifacts,omitempty"`
	Missing     []string `json:"missing,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}

// PhaseLedger indexes phase completion. Iterate in Phases order
// (never Go map order) when rendering or comparing for determinism.
type PhaseLedger map[Phase]PhaseLedgerEntry

// RegressEvent is the audit trail of one Brooks regression (stop →
// reflect → regress). Its count backs the regress cap: repeated stop/regress
// rounds on one cycle stop consuming tokens and escalate to a human decision.
type RegressEvent struct {
	Reason    string `json:"reason"`
	FromPhase Phase  `json:"from_phase"`
	At        string `json:"at"`
}

// DelegationContract is the scope a parent cycle hands to a child cycle: the
// task, its acceptance criteria and the parent plan it derives from.
type DelegationContract struct {
	ParentCycleID      string   `json:"parent_cycle_id"`
	TaskScope          string   `json:"task_scope"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	ParentPlanPath     string   `json:"parent_plan_path,omitempty"`
	ChildIssueURL      string   `json:"child_issue_url,omitempty"`
	DelegatedAt        string   `json:"delegated_at"`
}

// ChildCycleRef is the parent's reference to one child cycle, including the
// parent's validation verdict on that child's work.
type ChildCycleRef struct {
	CycleID            string   `json:"cycle_id"`
	Branch             string   `json:"branch"`
	Title              string   `json:"title,omitempty"`
	ChildIssueURL      string   `json:"child_issue_url,omitempty"`
	CreatedAt          string   `json:"created_at"`
	ValidationVerdict  string   `json:"validation_verdict,omitempty"`
	ValidationReason   string   `json:"validation_reason,omitempty"`
	ValidationEvidence []string `json:"validation_evidence,omitempty"`
	ValidatedAt        string   `json:"validated_at,omitempty"`
}

// ChildStartRequest asks the parent ParentID to create a child cycle on Branch.
type ChildStartRequest struct {
	ParentID           string
	Branch             string
	Title              string
	TaskScope          string
	AcceptanceCriteria []string
	ParentPlanPath     string
	ChildIssueURL      string
}

// ChildStartResult is the outcome of starting a child: the created child Record
// and the reference the parent stored for it.
type ChildStartResult struct {
	OK               bool          `json:"ok"`
	ParentID         string        `json:"parent_id"`
	Child            Record        `json:"child"`
	ParentRef        ChildCycleRef `json:"parent_ref"`
	Guidance         string        `json:"guidance,omitempty"`
	ChildLinkWarning string        `json:"child_link_warning,omitempty"`
}

// ChildStatusEntry is one child's phase and validation state as seen from the
// parent, with flags telling whether it came from the index, a scan, or neither.
type ChildStatusEntry struct {
	CycleID            string   `json:"cycle_id"`
	Branch             string   `json:"branch,omitempty"`
	Title              string   `json:"title,omitempty"`
	ChildIssueURL      string   `json:"child_issue_url,omitempty"`
	Phase              Phase    `json:"phase,omitempty"`
	LastActiveAt       string   `json:"last_active_at,omitempty"`
	WorktreePath       string   `json:"worktree_path,omitempty"`
	ValidationVerdict  string   `json:"validation_verdict,omitempty"`
	ValidationReason   string   `json:"validation_reason,omitempty"`
	ValidationEvidence []string `json:"validation_evidence,omitempty"`
	ValidatedAt        string   `json:"validated_at,omitempty"`
	ParentClosedState  string   `json:"parent_closed_state,omitempty"`
	Indexed            bool     `json:"indexed"`
	Scanned            bool     `json:"scanned"`
	Orphaned           bool     `json:"orphaned,omitempty"`
}

// ChildStatusResult lists a parent's children. Repaired and RepairAppended
// report index entries re-added during the call; Orphaned lists children whose
// parent no longer references them.
type ChildStatusResult struct {
	OK             bool               `json:"ok"`
	ParentID       string             `json:"parent_id"`
	Children       []ChildStatusEntry `json:"children"`
	Repaired       bool               `json:"repaired,omitempty"`
	RepairAppended []string           `json:"repair_appended,omitempty"`
	Orphaned       []string           `json:"orphaned,omitempty"`
}

// ChildValidationResult is the outcome of recording the parent's validation
// verdict for a child cycle.
type ChildValidationResult struct {
	OK        bool          `json:"ok"`
	ParentID  string        `json:"parent_id"`
	ChildID   string        `json:"child_id"`
	ParentRef ChildCycleRef `json:"parent_ref"`
}

// Record is the strict, versioned, durable state of one IssueOps cycle. It is
// the single source of truth that every phase gate reads and every phase
// transition writes; fields tagged json:"-" are in-memory load diagnostics.
type Record struct {
	OK                      bool                                `json:"ok"`
	Invalid                 bool                                `json:"-"`
	InvalidReason           string                              `json:"-"`
	SchemaVersion           int                                 `json:"schema_version"`
	ID                      string                              `json:"id"`
	Repo                    string                              `json:"repo"`
	Branch                  string                              `json:"branch,omitempty"`
	Phase                   Phase                               `json:"phase"`
	Intent                  *IntentContract                     `json:"intent,omitempty"`
	ArchitectureReview      *reviewcontract.ArchitectureReview  `json:"architecture_review,omitempty"`
	DomainReview            *DomainReview                       `json:"domain_review,omitempty"`
	IssueURL                string                              `json:"issue_url,omitempty"`
	ChildCreateOperations   []ChildCreateOperation              `json:"child_create_operations,omitempty"`
	IssueCreateIntent       *IssueCreateIntent                  `json:"issue_create_intent,omitempty"`
	PlanPath                string                              `json:"plan_path,omitempty"`
	WorktreePath            string                              `json:"worktree_path,omitempty"`
	IssueLinks              []IssueLink                         `json:"issue_links,omitempty"`
	BranchPrepare           *BranchPrepare                      `json:"branch_prepare,omitempty"`
	RemoteArtifact          *RemoteArtifactVerification         `json:"remote_artifact,omitempty"`
	BodySyncs               []RemoteBodySync                    `json:"body_syncs,omitempty"`
	Decisions               []Decision                          `json:"decisions,omitempty"`
	PlanPrep                *PlanPrep                           `json:"plan_prep,omitempty"`
	CompatibilityReview     *reviewcontract.CompatibilityReview `json:"compatibility_review,omitempty"`
	DevilsAdvocateReview    *DevilsAdvocateReview               `json:"devils_advocate_review,omitempty"`
	Feedback                []FeedbackItem                      `json:"feedback,omitempty"`
	RegressEvents           []RegressEvent                      `json:"regress_events,omitempty"`
	Delegation              *DelegationContract                 `json:"delegation,omitempty"`
	ChildCycles             []ChildCycleRef                     `json:"child_cycles,omitempty"`
	Execution               *Execution                          `json:"execution,omitempty"`
	RemoteCompletion        *RemoteCompletion                   `json:"remote_completion,omitempty"`
	SourceMisdirectWarnings int                                 `json:"source_misdirect_warnings,omitempty"`
	CleanupAttempt          *CleanupAttempt                     `json:"cleanup_attempt,omitempty"`
	CleanupFinishFailure    *CleanupFinishFailure               `json:"cleanup_finish_failure,omitempty"`
	LinkedBranchCleanup     *LinkedBranchCleanup                `json:"linked_branch_cleanup,omitempty"`
	CleanupAbandonFailure   *CleanupAbandonFailure              `json:"cleanup_abandon_failure,omitempty"`
	ImplementationReview    *ImplementationReview               `json:"implementation_review,omitempty"`
	ProjectDocsReview       *ProjectDocsReview                  `json:"project_docs_review,omitempty"`
	SchemaEvidence          *SchemaEvidence                     `json:"schema_evidence,omitempty"`
	RoutingTrace            []SkillRoutingEntry                 `json:"routing_trace,omitempty"`
	AISlopCleanAt           string                              `json:"ai_slop_clean_at,omitempty"`
	AISlopCleanHead         string                              `json:"ai_slop_clean_head,omitempty"`
	AISlopCleanFingerprint  string                              `json:"ai_slop_clean_fingerprint,omitempty"`
	AISlopCleanCategories   []string                            `json:"ai_slop_clean_categories,omitempty"`
	AISlopCleanVerification []string                            `json:"ai_slop_clean_verification,omitempty"`
	// PRHead is the canonical worktree HEAD sealed when the cycle entered pr, or when it
	// re-entered pr through the same strict gate after a post-pr fix. The gate requires a
	// clean, upstream-synced worktree with fresh review seals, so this is the verified head;
	// completion refuses a final head that changed anything but the verification report since.
	PRHead      string      `json:"pr_head,omitempty"`
	PhaseLedger PhaseLedger `json:"phase_ledger,omitempty"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
}

// ImplementationReview captures the architecture-review adversarial verdict on the
// implementation diff, recorded by the execution owner before publication.
// reviewer_* 필드는 감사 기록이며 게이트 조건이 아니다 — 하네스는 모델
// 자기신고를 검증할 수 없으므로 verdict와 실질 내용만 게이트한다(설계 v5 WS5).
type ImplementationReview struct {
	Verdict  string   `json:"verdict"` // pass | revise | stop
	Findings []string `json:"findings"`
	Evidence []string `json:"evidence"`
	// ReviewedFingerprint는 리뷰가 검토한 변경 집합의 content fingerprint다
	// (implementation.Reader.ChangeFingerprint — 커밋 후에도 안정). 게이트는 현재
	// fingerprint와 비교해 stale 리뷰를 거부한다(C4b-F1, ai_slop_clean 선례).
	ReviewedFingerprint string `json:"reviewed_fingerprint"`
	ReviewerHost        string `json:"reviewer_host,omitempty"`
	ReviewerModel       string `json:"reviewer_model,omitempty"`
	ReviewerEffort      string `json:"reviewer_effort,omitempty"`
	ReviewerContext     string `json:"reviewer_context,omitempty"`
	RecordedAt          string `json:"recorded_at"`
}

// ProjectDocsReview captures the pre-publication project-doc pass:
// 이번 변경이 CAUTIONS/ADR 같은 운영 문서에 남길 결정을 만들었는지 판정한 기록이다.
// verdict가 updated면 Docs에 적은 문서가 실제 변경 집합 안에 있어야 하므로,
// "갱신했다"는 자기신고만으로는 게이트를 통과할 수 없다.
type ProjectDocsReview struct {
	Verdict string   `json:"verdict"` // updated | no-change
	Docs    []string `json:"docs,omitempty"`
	// ReviewedDocs는 판정이 실제로 읽은 project doc의 repo-상대 경로다.
	// no-change는 최소 하나를 요구해 "대조했으나 없음"을 경로로 증명한다.
	ReviewedDocs []string `json:"reviewed_docs,omitempty"`
	Evidence     []string `json:"evidence"`
	// ReviewedFingerprint는 검토가 본 변경 집합의 content fingerprint다
	// (implementation_review 선례). 이후 diff가 바뀌면 stale로 거부한다.
	ReviewedFingerprint string `json:"reviewed_fingerprint"`
	RecordedAt          string `json:"recorded_at"`
}

// SchemaEvidence 타입은 스키마·마이그레이션·엔티티 변경이 변경 집합에
// 들어 있을 때만 요구되는 실측 근거다. 인덱스 현황, row count처럼 실제
// 데이터베이스에서 관찰한 값과 그 출처를 남긴다. 관찰이 불가능한 상황은
// 근거를 적어 waive한다.
type SchemaEvidence struct {
	Measurements        []string `json:"measurements,omitempty"`
	Sources             []string `json:"sources,omitempty"`
	Waived              bool     `json:"waived,omitempty"`
	WaiverRationale     string   `json:"waiver_rationale,omitempty"`
	ReviewedFingerprint string   `json:"reviewed_fingerprint"`
	RecordedAt          string   `json:"recorded_at"`
}

// CleanupFinishFailure marks the step where a destructive cleanup
// apply stopped. finish 재실행이 이어받을 수 있도록 레코드에 남긴다(resumable).
const (
	CleanupFailureStepApplying       = "applying"
	CleanupFailureStepWorktreeRemove = "worktree_remove"
	CleanupFailureStepBranchDelete   = "branch_delete"
	CleanupFailureStepRecordDelete   = "record_delete"
	// CleanupFailureStepWorkspaceProcessesStop은 apply ①′(워크트리 점유 프로세스 종료)이 재관측에서 점유 0을 증명하지 못한 지점이다(#477).
	CleanupFailureStepWorkspaceProcessesStop = "workspace_processes_stop"
	// 아래 셋은 abandon의 원격 효과 단계다. 로컬 단계보다 먼저 실행되므로
	// 여기서 멈추면 레코드도 워크트리도 그대로 남는다 — 원격만 부분적으로
	// 바뀐 상태를 사람이 보고 다시 결정할 수 있어야 한다.
	CleanupFailureStepClosePR            = "close_pr"
	CleanupFailureStepCloseIssue         = "close_issue"
	CleanupFailureStepRemoteBranchDelete = "remote_branch_delete"
)

// CleanupFinishFailure records where a destructive cleanup apply stopped, so a
// finish rerun can resume from Step.
type CleanupFinishFailure struct {
	Step    string `json:"step"`
	Message string `json:"message"`
	At      string `json:"at"`
}

// CleanupAbandonFailure 타입은 부분 실패 뒤 같은 로컬 대상을 재시도할 수 있도록
// 마지막으로 승인된 inventory를 봉인한다.
type CleanupAbandonFailure struct {
	Step            string `json:"step"`
	Message         string `json:"message"`
	Fingerprint     string `json:"fingerprint"`
	RecordSHA       string `json:"record_sha"`
	InventorySHA256 string `json:"inventory_sha256"`
	WorktreePath    string `json:"worktree_path"`
	Branch          string `json:"branch"`
	WorktreeHead    string `json:"worktree_head"`
	BranchOID       string `json:"branch_oid"`
	At              string `json:"at"`
}

// RemoteCompletion caches confirmed remote completion mutations.
// 원격 readback이 항상 우선 판정이고 이 필드는 보조 캐시다(설계 v5 WS3).
type RemoteCompletion struct {
	ReflectedAt   string `json:"reflected_at,omitempty"`
	IssueClosedAt string `json:"issue_closed_at,omitempty"`
	// MergedAt is when remote merge-pr read the PR/MR back merged, so next
	// can route past the merge without reading the provider.
	MergedAt string `json:"merged_at,omitempty"`
}

// SkillRoutingEntry records that a pioneer/CS skill fired at a given IssueOps
// phase during a real run. Captured live, it lets skill_routing_fidelity be
// scored against observed activation instead of a synthesized trace.
type SkillRoutingEntry struct {
	Phase string `json:"phase"`
	Skill string `json:"skill"`
	At    string `json:"at,omitempty"`
}

// Readiness reports whether a cycle may enter its next gate, with the Missing
// requirements that block it and the heads/fingerprints the check compared.
type Readiness struct {
	OK                     bool     `json:"ok"`
	Ready                  bool     `json:"ready"`
	Strict                 bool     `json:"strict,omitempty"`
	Missing                []string `json:"missing"`
	IssueURL               string   `json:"issue_url,omitempty"`
	PlanPath               string   `json:"plan_path,omitempty"`
	WorktreePath           string   `json:"worktree_path,omitempty"`
	Branch                 string   `json:"branch,omitempty"`
	AISlopCleanHead        string   `json:"ai_slop_clean_head,omitempty"`
	CurrentHead            string   `json:"current_head,omitempty"`
	AISlopCleanFingerprint string   `json:"ai_slop_clean_fingerprint,omitempty"`
	CurrentFingerprint     string   `json:"current_fingerprint,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
}

// CleanupStatusRequest asks for cleanup readiness; Merged is true only when the
// caller has read the remote merge back.
type CleanupStatusRequest struct {
	Merged bool `json:"merged"`
}

// CleanupStatus is the cleanup readiness report: what is missing, the choices
// available, and the remote artifact the decision rests on.
type CleanupStatus struct {
	OK                bool     `json:"ok"`
	Ready             bool     `json:"ready"`
	ID                string   `json:"id"`
	Merged            bool     `json:"merged"`
	Missing           []string `json:"missing"`
	Warnings          []string `json:"warnings,omitempty"`
	Choices           []string `json:"choices"`
	WorktreePath      string   `json:"worktree_path,omitempty"`
	Branch            string   `json:"branch,omitempty"`
	RemoteArtifactURL string   `json:"remote_artifact_url,omitempty"`
	// NextCommand is the remedy for the first blocking missing key, or the
	// finish preview once ready; set only after the merge readback (--merged).
	NextCommand string `json:"next_command,omitempty"`
}

// CloseChildrenRequest asks to close a parent's child issues. Without Confirm
// the call is a preview.
type CloseChildrenRequest struct {
	// Merged는 부모 PR/MR의 머지가 원격 readback으로 검증됐음을 뜻한다.
	Merged bool `json:"merged"`
	// MergeEvidenceRequested는 운영자가 --merged로 정리 의도를 명시했다는
	// 뜻이다. Merged와 다르다: 위상 규약 이전의 우산 레코드는 자체 PR이 없어
	// 부모 머지 증거를 만들 수 없으므로 Merged가 false로 남는다. 그 구간에서만
	// 자식의 원격 closed 상태를 대체 증거로 조회한다(#129).
	MergeEvidenceRequested bool `json:"merge_evidence_requested,omitempty"`
	Confirm                bool `json:"confirm"`
}

// CloseChildResult is the outcome (or preview) of closing one child issue.
type CloseChildResult struct {
	URL               string `json:"url"`
	Provider          string `json:"provider,omitempty"`
	Closed            bool   `json:"closed"`
	AlreadyClosed     bool   `json:"already_closed,omitempty"`
	HierarchyVerified bool   `json:"hierarchy_verified"`
	State             string `json:"state,omitempty"`
	Preview           string `json:"preview,omitempty"`
	Error             string `json:"error,omitempty"`
}

// CloseChildrenResult is the outcome of closing a parent's children, including
// the evidence basis that let the gate pass.
type CloseChildrenResult struct {
	OK        bool   `json:"ok"`
	ID        string `json:"id"`
	Merged    bool   `json:"merged"`
	Confirmed bool   `json:"confirmed"`
	DryRun    bool   `json:"dry_run"`
	// EvidenceBasis는 이 실행이 무엇을 근거로 게이트를 통과했는지다.
	// parent_merge_verified는 부모 PR/MR의 머지 readback이고,
	// children_already_closed는 모든 자식이 원격에서 이미 닫혀 있다는 관측이다.
	// 운영자가 어느 근거로 정리됐는지 알아야 하므로 결과에 남긴다.
	EvidenceBasis string             `json:"evidence_basis,omitempty"`
	ClosedCount   int                `json:"closed_count"`
	Children      []CloseChildResult `json:"children"`
	Missing       []string           `json:"missing,omitempty"`
}

// LeaseHolderIndex is the index entry mapping a lifecycle to the session
// holding its write lease, so a holder can be found without loading every record.
type LeaseHolderIndex struct {
	Key           string `json:"-"`
	SchemaVersion int    `json:"schema_version"`
	LifecycleID   string `json:"lifecycle_id"`
	Generation    uint64 `json:"generation"`
	Host          string `json:"host"`
	SessionID     string `json:"session_id"`
	AgentID       string `json:"agent_id,omitempty"`
}

// CleanupRemoteBranchArtifactHead 타입은 머지 검증 readback이 함께 돌려주는 원격
// PR/MR의 head ref 정체다. 게이트 ⑨(타 브랜치 PR 방어)와 ⑩(머지 후 push된
// 커밋 유실 방지)이 이 두 값에만 의존한다.
type CleanupRemoteBranchArtifactHead struct {
	HeadRefName string
	HeadRefOID  string
	// BaseRefName은 같은 readback에서 관측한 artifact의 현재 base ref다.
	// remote-branch 게이트는 이 값을 읽지 않지만, cleanup finish의 base drift
	// 게이트가 머지 관측과 같은 시점의 base를 요구하므로 여기에 함께 실린다.
	BaseRefName string
}

// LinkedBranchCleanup 타입은 ref-null 고아 linked-branch 처분의 durable
// audit이다(#306 AC-06). 성공만이 아니라 거절도 남긴다 — 다음 사람이 "왜 아직
// 안 지워졌나"를 처음부터 다시 조사하지 않게 하는 것이 이 기록의 목적이다.
type LinkedBranchCleanup struct {
	State          string `json:"state"`
	StateReason    string `json:"state_reason,omitempty"`
	LinkedBranchID string `json:"linked_branch_id,omitempty"`
	LinkedCount    int    `json:"linked_count"`
	RemoteRefOID   string `json:"remote_ref_oid,omitempty"`
	Fingerprint    string `json:"fingerprint,omitempty"`
	Deleted        bool   `json:"deleted,omitempty"`
	AlreadyAbsent  bool   `json:"already_absent,omitempty"`
	FailedStep     string `json:"failed_step,omitempty"`
	ObservedAt     string `json:"observed_at"`
}

// TrackedMaterials reports copied implementation materials. This response
// is not persisted in the strict versioned IssueOps record.
type TrackedMaterials struct {
	Written  []string `json:"written,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}
