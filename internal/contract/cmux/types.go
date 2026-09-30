package cmux

import model "issueops/internal/contract/issueops"

type PreflightRequest struct {
	Executable      string
	ExpectedVersion string
	ExpectedBuild   string
	SocketPath      string
	WindowID        string
}

type PreflightResult struct {
	Executable string
	Version    string
	Build      string
	SocketPath string
	WindowID   string
	Endpoint   model.IssueOpsHandoffDeliveryEndpointIncarnation
}

type CreateRequest struct {
	Preflight PreflightResult
	AttemptID string
	CWD       string
}

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

type SendRequest struct {
	Created CreatedWorkspace
	Command string
}

type SendReceipt struct {
	Accepted bool
	SendMS   uint64
}

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

type PreparedLauncher struct {
	Directory      string
	PromptPath     string
	LauncherPath   string
	ReceiptPath    string
	Command        string
	HostArgvSHA256 string
}

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
