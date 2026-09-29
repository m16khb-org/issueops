package issueops

type OwnerIssue struct {
	URL        string `json:"url"`
	Body       string `json:"body"`
	BodySHA256 string `json:"body_sha256"`
}

type OwnerCommands struct {
	LeaseStatus          string `json:"lease_status"`
	Claim                string `json:"claim"`
	AwaitBranchLink      string `json:"await_branch_link"`
	Release              string `json:"release"`
	VerifyBranchLinkRead string `json:"verify_branch_link_read"`
	VerifyBranchLink     string `json:"verify_branch_link"`
	LinkPlan             string `json:"link_plan"`
	CompatibilityReview  string `json:"compatibility_review"`
	EnterImplement       string `json:"enter_implement"`
	AISlopCleanRecord    string `json:"ai_slop_clean_record"`
	EnterAISlopClean     string `json:"enter_ai_slop_clean"`
	RemoteCreate         string `json:"remote_create"`
	Complete             string `json:"complete"`
	ImplementationReview string `json:"implementation_review"`
	ProjectDocsReview    string `json:"project_docs_review"`
	SchemaEvidence       string `json:"schema_evidence"`
	EnterPR              string `json:"enter_pr"`
}

type OwnerContextPacket struct {
	SchemaVersion          int               `json:"schema_version"`
	LifecycleID            string            `json:"lifecycle_id"`
	Mode                   ExecutionMode     `json:"mode"`
	SourceRoot             string            `json:"source_root"`
	WorktreeRoot           string            `json:"worktree_root"`
	WorktreeBase           string            `json:"worktree_base"`
	Branch                 string            `json:"branch"`
	BaseHead               string            `json:"base_head"`
	CurrentHead            string            `json:"current_head"`
	LeaseGeneration        uint64            `json:"lease_generation"`
	Issue                  OwnerIssue        `json:"issue"`
	OwnerHost              string            `json:"owner_host"`
	OwnerModel             string            `json:"owner_model"`
	OwnerEffort            string            `json:"owner_effort,omitempty"`
	ReviewerModel          string            `json:"reviewer_model,omitempty"`
	ReviewerEffort         string            `json:"reviewer_effort,omitempty"`
	ResearchModel          string            `json:"research_model,omitempty"`
	ResearchEffort         string            `json:"research_effort,omitempty"`
	RequiredDocs           []string          `json:"required_docs"`
	RequiredSkills         []string          `json:"required_skills"`
	AcceptanceIDs          []string          `json:"acceptance_ids"`
	Verification           []string          `json:"verification_commands"`
	VerificationReportPath string            `json:"verification_report_path"`
	ArtifactManifest       map[string]string `json:"artifact_manifest,omitempty"`
	Commands               OwnerCommands     `json:"commands"`
}

type OwnerSnapshot struct {
	Issue                OwnerIssue
	RequiredDocs         []string
	RequiredSkills       []string
	AcceptanceIDs        []string
	VerificationCommands []string
}

type OwnerArtifacts struct {
	PacketPath   string
	PacketSHA256 string
	PromptPath   string
	PromptSHA256 string
	Prompt       string
}

type OwnerPaths struct {
	Packet       string
	Prompt       string
	Plan         string
	Report       string
	WorktreeBase string
}
type OwnerPlanIdentity struct {
	Path   string
	Digest string
}

// OwnerPolicyContext carries decisions from the model and remote identity domains.
type OwnerPolicyContext struct {
	ReviewerModel, ReviewerEffort, ResearchModel, ResearchEffort string
	ProjectKey, IssueNumber                                      string
}
