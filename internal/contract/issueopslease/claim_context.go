package issueopslease

type ClaimContextPacket struct {
	SchemaVersion    int               `json:"schema_version"`
	LifecycleID      string            `json:"lifecycle_id"`
	Mode             string            `json:"mode"`
	SourceRoot       string            `json:"source_root"`
	WorktreeRoot     string            `json:"worktree_root"`
	Branch           string            `json:"branch"`
	BaseHead         string            `json:"base_head"`
	LeaseGeneration  uint64            `json:"lease_generation"`
	Issue            ClaimPacketIssue  `json:"issue"`
	ArtifactManifest map[string]string `json:"artifact_manifest"`
}

type ClaimPacketIssue struct {
	URL        string `json:"url"`
	Body       string `json:"body"`
	BodySHA256 string `json:"body_sha256"`
}
