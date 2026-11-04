// Package cmux defines the request and result types exchanged with the cmux terminal multiplexer port during
// handoffs: preflight, workspace creation, input delivery, and launcher artifacts. It holds data only.
package cmux

import model "issueops/internal/contract/issueops"

// PreflightRequest describes the cmux host a handoff expects: the executable with its expected version and build,
// and optionally the socket path and window to pin.
type PreflightRequest struct {
	Executable      string
	ExpectedVersion string
	ExpectedBuild   string
	SocketPath      string
	WindowID        string
}

// PreflightResult is the verified identity of the cmux host found by a preflight, including the incarnation
// of its socket endpoint so later steps can detect a replaced endpoint.
type PreflightResult struct {
	Executable string
	Version    string
	Build      string
	SocketPath string
	WindowID   string
	Endpoint   model.HandoffDeliveryEndpointIncarnation
}

// CreateRequest asks for a new workspace in the host that Preflight verified, tagged with the handoff
// attempt ID and started in CWD.
type CreateRequest struct {
	Preflight PreflightResult
	AttemptID string
	CWD       string
}

// CreatedWorkspace identifies a workspace that was created: its window, workspace, pane, and surface IDs,
// the working directory, and how long creating and resolving it took in milliseconds.
type CreatedWorkspace struct {
	Preflight   PreflightResult
	WindowID    string
	WorkspaceID string
	PaneID      string
	SurfaceID   string
	CWD         string
	CreateMS    uint64
	ResolveMS   uint64
}

// SendRequest sends Command as input to the surface of a created workspace.
type SendRequest struct {
	Created CreatedWorkspace
	Command string
}

// SendReceipt reports whether the host accepted the input and how long sending took in milliseconds.
type SendReceipt struct {
	Accepted bool
	SendMS   uint64
}

// ArtifactRequest holds everything needed to write a launcher artifact: the artifact root, the target
// workspace identity, the host, model, and effort to launch, and the prompt with the digests that bind it
// and its source material. Prompt must match PromptSHA256.
type ArtifactRequest struct {
	Root           string
	CWD            string
	WindowID       string
	WorkspaceID    string
	SurfaceID      string
	SocketPath     string
	Host           string
	HostExecutable string
	Model          string
	Effort         string
	Prompt         []byte
	PromptSHA256   string
	MaterialSHA256 string
}

// PreparedLauncher locates a launcher artifact on disk: its directory, the prompt, launcher script, and
// bootstrap receipt files, the shell command that runs it, and the digest of the host argv.
type PreparedLauncher struct {
	Directory      string
	PromptPath     string
	LauncherPath   string
	ReceiptPath    string
	Command        string
	HostArgvSHA256 string
}

// BootstrapExpectation is the identity a bootstrap receipt must match to prove the launched host
// started in the intended workspace with the intended argv, prompt, and material.
type BootstrapExpectation struct {
	CWD            string
	WindowID       string
	WorkspaceID    string
	SurfaceID      string
	SocketPath     string
	HostExecutable string
	HostArgvSHA256 string
	PromptSHA256   string
	MaterialSHA256 string
}
