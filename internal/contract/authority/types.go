// Package authority holds the authority capability's DTOs: the stored grant
// record, issue requests and receipts, the request-local credential use, and the
// resolved workspace scope.
package authority

import (
	model "issueops/internal/contract/issueops"
	statecontract "issueops/internal/contract/state"
)

// SchemaVersion is the version of the stored Record layout. Bucket names the state
// bucket that holds grant records. CodeRequired and CodeInvalid are the error
// codes for a missing and for an unusable authority credential.
const (
	SchemaVersion = 1
	Bucket        = "issueops_authority_v1"

	CodeRequired = "authority_required"
	CodeInvalid  = "authority_invalid"
)

// NativeActor is the capability-local alias of the IssueOps native actor; it keeps
// domain/authority on its own contract package.
type NativeActor = model.NativeActor

// ErrInvalidState is the shared fail-closed schema error, named here because
// domain/authority may import only this contract package.
var ErrInvalidState = statecontract.ErrInvalidState

// Record is a stored authority grant: the actor it was issued to, the workspace
// it is scoped to, and the SHA-256 of the token. The token itself is never
// stored. IssuedAt and ExpiresAt are RFC 3339 timestamps.
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

// IssueRequest asks for a new authority credential for Actor within
// WorkspaceRoot.
type IssueRequest struct {
	WorkspaceRoot string            `json:"workspace_root"`
	Actor         model.NativeActor `json:"actor"`
}

// Receipt reports an issued grant: the credential file holding the token and when
// the grant expires.
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

// Scope is the resolved location a grant applies to: the requested workspace and
// working directory, plus the source root and Git common directory they belong to.
type Scope struct {
	WorkspaceRoot string
	CWD           string
	SourceRoot    string
	GitCommonDir  string
}
