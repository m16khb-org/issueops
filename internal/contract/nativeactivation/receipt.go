// Package nativeactivation holds the contract DTOs of the native activation
// transition: the request, the readback evidence, the sealed receipt, and the
// result. It is a contract package with no behavior.
package nativeactivation

// SchemaVersion is the version stamped on every sealed Receipt.
const SchemaVersion = 1

// Request names the state root, harness root, and target binary of an
// activation. TransitionID is empty for begin and required for seal and abort.
type Request struct {
	StateRoot    string `json:"state_root"`
	IssueOpsRoot string `json:"issueops_root"`
	TargetBinary string `json:"target_binary"`
	TransitionID string `json:"transition_id,omitempty"`
}

// Evidence is the readback fact for one host surface (such as codex hooks):
// the file path, its semantic and raw SHA-256 digests, and the file identity
// (mode, size, device, inode) observed at readback.
type Evidence struct {
	Host           string `json:"host"`
	Surface        string `json:"surface"`
	Path           string `json:"path"`
	SemanticSHA256 string `json:"semantic_sha256"`
	SHA256         string `json:"sha256"`
	Mode           uint32 `json:"mode"`
	Size           int64  `json:"size"`
	Device         uint64 `json:"device"`
	Inode          uint64 `json:"inode"`
}

// Receipt is the sealed record of a completed activation: the binary and
// catalog digests, the readback Evidence, and when it was sealed.
type Receipt struct {
	SchemaVersion int        `json:"schema_version"`
	StateRoot     string     `json:"state_root"`
	IssueOpsRoot  string     `json:"issueops_root"`
	TargetBinary  string     `json:"target_binary"`
	BinarySHA256  string     `json:"binary_sha256"`
	TransitionID  string     `json:"transition_id"`
	CatalogSHA256 string     `json:"catalog_sha256"`
	Evidence      []Evidence `json:"evidence"`
	SealedAt      string     `json:"sealed_at"`
}

// Result reports the state of one transition. Exactly one of Pending, Sealed,
// and Aborted is true on success; Receipt is set only when Sealed.
type Result struct {
	OK           bool     `json:"ok"`
	StateRoot    string   `json:"state_root"`
	IssueOpsRoot string   `json:"issueops_root"`
	TargetBinary string   `json:"target_binary"`
	BinarySHA256 string   `json:"binary_sha256,omitempty"`
	TransitionID string   `json:"transition_id"`
	Pending      bool     `json:"pending"`
	Sealed       bool     `json:"sealed"`
	Aborted      bool     `json:"aborted,omitempty"`
	UpdatedAt    string   `json:"updated_at"`
	Receipt      *Receipt `json:"receipt,omitempty"`
}
