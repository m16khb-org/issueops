package issueops

import "context"

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

type ExecutionCmuxHandoffResult struct {
	OK                  bool                            `json:"ok"`
	ID                  string                          `json:"id"`
	Generation          uint64                          `json:"generation"`
	Status              string                          `json:"status"`
	Launcher            IssueOpsHandoffDeliveryLauncher `json:"launcher"`
	Target              IssueOpsHandoffDeliveryTarget   `json:"target"`
	Timing              *IssueOpsHandoffDeliveryTiming  `json:"timing,omitempty"`
	InputAccepted       bool                            `json:"input_accepted"`
	NativeTurnObserved  bool                            `json:"native_turn_observed"`
	OwnerClaimed        bool                            `json:"owner_claimed"`
	RecoveryArtifactDir string                          `json:"recovery_artifact_dir,omitempty"`
	ObservationReceipt  IssueOpsHandoffDeliveryReceipt  `json:"observation_receipt"`
}

type ExecutionCmuxHandoffHandler func(context.Context, string, ExecutionCmuxHandoffRequest) (ExecutionCmuxHandoffResult, error)
