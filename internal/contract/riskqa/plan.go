package riskqa

// TierPlan is the verification plan chosen for a change: the risk Tier, the
// ChangedPaths and Reasons that led to it, the Commands to run, and the
// resolved tree Scope when it could be determined.
type TierPlan struct {
	Tier         string   `json:"tier"`
	ChangedPaths []string `json:"changed_paths"`
	Reasons      []string `json:"reasons"`
	Commands     []string `json:"commands"`
	Scope        *Scope   `json:"scope,omitempty"`
}

// Scope records resolved tree boundaries and observation failures.
type Scope struct {
	BaseSHA string `json:"base_sha,omitempty"`
	HeadSHA string `json:"head_sha,omitempty"`
	Error   string `json:"error,omitempty"`
}
