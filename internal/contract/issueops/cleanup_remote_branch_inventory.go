package issueops

// CleanupRemoteBranchInventory binds confirmation to the observed remote tip
// and the artifact used to justify deletion. Field order is fingerprinted.
type CleanupRemoteBranchInventory struct {
	ID           string `json:"id"`
	Repo         string `json:"repo"`
	Branch       string `json:"branch"`
	RemoteOID    string `json:"remote_oid"`
	ArtifactURL  string `json:"artifact_url"`
	SupersededBy string `json:"superseded_by,omitempty"`
}
