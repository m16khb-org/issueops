// Package lifecycle defines the project lifecycle DTOs: fingerprint, stored
// profile, and the resolved state plan. It sits in the contract layer.
package lifecycle

import projectdoccontract "issueops/internal/contract/projectdoc"

// ProjectFingerprint identifies a repository by root, git dir, and a hash of
// its origin URL, so state can be keyed to the project rather than a path.
type ProjectFingerprint struct {
	RepoRoot      string `json:"repo_root"`
	GitDir        string `json:"git_dir,omitempty"`
	GitOriginHash string `json:"git_origin_hash,omitempty"`
}

// ProjectLifecycleProfile is the project.json record persisted per project,
// with RFC 3339 CreatedAt and UpdatedAt timestamps.
type ProjectLifecycleProfile struct {
	SchemaVersion int                                `json:"schema_version"`
	RepoID        string                             `json:"repo_id"`
	Fingerprint   ProjectFingerprint                 `json:"fingerprint"`
	Metadata      *projectdoccontract.ProjectProfile `json:"metadata,omitempty"`
	CreatedAt     string                             `json:"created_at"`
	UpdatedAt     string                             `json:"updated_at"`
}

// ProjectLifecycleStatePlan reports where a project's lifecycle state lives and
// whether its profile exists with a valid namespace. OK is false when the plan
// could not be resolved; Warnings carries non-fatal read problems.
type ProjectLifecycleStatePlan struct {
	OK              bool                     `json:"ok"`
	SchemaVersion   int                      `json:"schema_version"`
	RepoRoot        string                   `json:"repo_root"`
	RepoID          string                   `json:"repo_id"`
	StateRoot       string                   `json:"state_root"`
	ProjectStateDir string                   `json:"project_state_dir"`
	ProjectJSONPath string                   `json:"project_json_path"`
	QueuePath       string                   `json:"queue_path"`
	CompactPath     string                   `json:"compact_path"`
	Fingerprint     ProjectFingerprint       `json:"fingerprint"`
	Exists          bool                     `json:"exists"`
	NamespaceValid  bool                     `json:"namespace_valid"`
	Profile         *ProjectLifecycleProfile `json:"profile,omitempty"`
	Warnings        []string                 `json:"warnings,omitempty"`
}
