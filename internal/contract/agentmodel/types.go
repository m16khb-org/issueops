// Package agentmodel holds the wire types for role-based agent model settings.
package agentmodel

// Role names the purpose an IssueOps session or sub-agent runs for.
type Role string

const (
	RoleImplement      Role = "implement"
	RoleChildImplement Role = "child-implement"
	RolePlanReview     Role = "plan-review"
	RoleDiffReview     Role = "diff-review"
	RoleReviewEscalate Role = "review-escalate"
	RoleResearch       Role = "research"
	RoleReaderCheck    Role = "reader-check"
)

// Layer is one settings layer for a role. Empty fields fall through to the
// next layer.
type Layer struct {
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
}

// ConfigVersion is the only settings file version this build reads.
const ConfigVersion = 1

// Config is the content of a global or local agent-models.json file.
type Config struct {
	Version int            `json:"version"`
	Claude  map[Role]Layer `json:"claude,omitempty"`
	Codex   map[Role]Layer `json:"codex,omitempty"`
}

// Source labels where a resolved field came from.
const (
	SourceFlag      = "flag"
	SourceLocal     = "local"
	SourceGlobal    = "global"
	SourceDefault   = "default"
	SourceInherited = "inherited"
)

// Resolution is the final model and effort for one host and role.
type Resolution struct {
	Host         string `json:"host"`
	Role         Role   `json:"role"`
	Model        string `json:"model"`
	Effort       string `json:"effort"`
	ModelSource  string `json:"model_source"`
	EffortSource string `json:"effort_source"`
}
