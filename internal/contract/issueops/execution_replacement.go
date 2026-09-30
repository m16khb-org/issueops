package issueops

import (
	executionissue "issueops/internal/contract/executionissue"
)

type ExecutionReplaceRequest struct {
	ID                    string                                        `json:"id"`
	Action                string                                        `json:"action"`
	ExpectedGeneration    uint64                                        `json:"expected_generation"`
	CompletionGeneration  uint64                                        `json:"completion_generation,omitempty"`
	InventoryFingerprint  string                                        `json:"inventory_fingerprint,omitempty"`
	QuiescenceFingerprint string                                        `json:"quiescence_fingerprint,omitempty"`
	Reason                string                                        `json:"reason,omitempty"`
	Actor                 NativeActor                                   `json:"actor"`
	CWD                   string                                        `json:"cwd"`
	ReadIssue             executionissue.ExecutionIssueSnapshotReadFunc `json:"-"`
	Confirm               bool                                          `json:"confirm,omitempty"`
}

type ReplacementArtifacts struct {
	IssueBodySHA256     string
	ContextPacketPath   string
	ContextPacketSHA256 string
	OwnerPromptPath     string
	OwnerPromptSHA256   string
}

type ReplacementWorkspaceProcess struct {
	PID     int
	Command string
	FD      string
	Access  string
	Path    string
}
