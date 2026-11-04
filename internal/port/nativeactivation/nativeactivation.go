// Package nativeactivation defines the port for the native activation transaction that makes a freshly
// installed issueops binary live: begin records a pending transition, seal commits it with host readback evidence,
// and abort cancels it. The state-store backend and the host readback verifier implement these interfaces.
package nativeactivation

import "context"

// Evidence is one host configuration file observed during readback: the host and surface it belongs to, its path,
// the semantic and raw SHA-256 digests, and the file mode, size, device, and inode.
type Evidence struct {
	Host           string
	Surface        string
	Path           string
	SemanticSHA256 string
	SHA256         string
	Mode           uint32
	Size           int64
	Device         uint64
	Inode          uint64
}

// Readback is the result of verifying host configuration against a binary: the catalog digest and the evidence files it was derived from.
type Readback struct {
	CatalogSHA256 string
	Evidence      []Evidence
}

// BeginRequest starts an activation transition for TargetBinary under the given state and issueops roots.
type BeginRequest struct {
	StateRoot    string
	IssueOpsRoot string
	TargetBinary string
}

// SealRequest commits the pending transition TransitionID, binding it to the verified CatalogSHA256 and host Evidence.
type SealRequest struct {
	StateRoot     string
	IssueOpsRoot  string
	TargetBinary  string
	TransitionID  string
	CatalogSHA256 string
	Evidence      []Evidence
}

// AbortRequest cancels the pending transition TransitionID.
type AbortRequest struct {
	StateRoot    string
	IssueOpsRoot string
	TargetBinary string
	TransitionID string
}

// Result is the stored activation state after Begin, Seal, or Abort: the roots and binary digest, the TransitionID,
// whether it is Pending, Sealed, or Aborted, and when it last changed.
type Result struct {
	StateRoot    string
	IssueOpsRoot string
	TargetBinary string
	BinarySHA256 string
	TransitionID string
	Pending      bool
	Sealed       bool
	Aborted      bool
	UpdatedAt    string
}

// Backend persists activation transitions. Begin creates a pending transition, Seal commits it, and Abort cancels it.
type Backend interface {
	Begin(context.Context, BeginRequest) (Result, error)
	Seal(context.Context, SealRequest) (Result, error)
	Abort(context.Context, AbortRequest) (Result, error)
}

// ReadbackVerifier reads the host configurations back and verifies them for the given issueops root and binary, returning the evidence Seal needs.
type ReadbackVerifier interface {
	Verify(context.Context, string, string) (Readback, error)
}
