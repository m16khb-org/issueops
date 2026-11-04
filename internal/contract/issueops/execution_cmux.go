package issueops

import "context"

// ExecutionCmuxHandoffRequest asks to hand the execution lease at Generation
// over to a new agent session launched through cmux. It pins the cmux build,
// the host, and the SHA-256 of the prompt and materials so the delivery can be
// verified afterwards.
type ExecutionCmuxHandoffRequest struct {
	ID             string `json:"id"`
	Generation     uint64 `json:"generation"`
	CmuxExecutable string `json:"cmux_executable"`
	CmuxVersion    string `json:"cmux_version"`
	CmuxBuild      string `json:"cmux_build_identity"`
	SocketPath     string `json:"socket_path"`
	WindowID       string `json:"window_id"`
	CWD            string `json:"cwd"`
	Host           string `json:"host"`
	HostExecutable string `json:"host_executable"`
	Model          string `json:"model"`
	Effort         string `json:"effort,omitempty"`
	PromptFile     string `json:"prompt_file"`
	PromptSHA256   string `json:"prompt_sha256"`
	MaterialSHA256 string `json:"material_sha256"`
}

// ExecutionCmuxHandoffResult reports a cmux handoff: where it was delivered, how
// far delivery progressed (input accepted, native turn observed, owner claimed),
// and the receipt behind that observation.
type ExecutionCmuxHandoffResult struct {
	OK                  bool                    `json:"ok"`
	ID                  string                  `json:"id"`
	Generation          uint64                  `json:"generation"`
	Status              string                  `json:"status"`
	Launcher            HandoffDeliveryLauncher `json:"launcher"`
	Target              HandoffDeliveryTarget   `json:"target"`
	Timing              *HandoffDeliveryTiming  `json:"timing,omitempty"`
	InputAccepted       bool                    `json:"input_accepted"`
	NativeTurnObserved  bool                    `json:"native_turn_observed"`
	OwnerClaimed        bool                    `json:"owner_claimed"`
	RecoveryArtifactDir string                  `json:"recovery_artifact_dir,omitempty"`
	ObservationReceipt  HandoffDeliveryReceipt  `json:"observation_receipt"`
}

// ExecutionCmuxHandoffHandler runs a cmux handoff for the cycle with the given ID.
type ExecutionCmuxHandoffHandler func(context.Context, string, ExecutionCmuxHandoffRequest) (ExecutionCmuxHandoffResult, error)
