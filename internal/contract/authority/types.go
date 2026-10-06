package authority

import (
	model "issueops/internal/contract/issueops"
	statecontract "issueops/internal/contract/state"
)

const (
	SchemaVersion = 1
	Bucket        = "issueops_authority_v1"

	CodeRequired = "authority_required"
	CodeInvalid  = "authority_invalid"
)

// Capability-local aliases keep domain/authority on its own contract package.
type NativeActor = model.NativeActor

// ErrInvalidState is the shared fail-closed schema error.
var ErrInvalidState = statecontract.ErrInvalidState

type Record struct {
	SchemaVersion int               `json:"schema_version"`
	Key           string            `json:"key"`
	SourceRoot    string            `json:"source_root"`
	GitCommonDir  string            `json:"git_common_dir"`
	Actor         model.NativeActor `json:"actor"`
	TokenSHA256   string            `json:"token_sha256"`
	IssuedAt      string            `json:"issued_at"`
	ExpiresAt     string            `json:"expires_at"`
}

type IssueRequest struct {
	WorkspaceRoot string            `json:"workspace_root"`
	Actor         model.NativeActor `json:"actor"`
}

type Receipt struct {
	OK            bool   `json:"ok"`
	AuthorityFile string `json:"authority_file"`
	ExpiresAt     string `json:"expires_at"`
}

// Use carries request-local authority. The credential never crosses a JSON boundary.
type Use struct {
	Key           string `json:"key"`
	Token         string `json:"-"`
	WorkspaceRoot string `json:"workspace_root"`
	CWD           string `json:"cwd"`
	Tool          string `json:"tool"`
	Action        string `json:"action"`
}

type Scope struct {
	WorkspaceRoot string
	CWD           string
	SourceRoot    string
	GitCommonDir  string
}
