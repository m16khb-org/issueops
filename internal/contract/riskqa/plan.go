package riskqa

type RiskQATierPlan struct {
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
