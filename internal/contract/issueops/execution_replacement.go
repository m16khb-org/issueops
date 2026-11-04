package issueops

// ExecutionReplaceRequest asks to replace the holder of the write lease.
// Action selects the step (preview, revoke, finalize-preview, finalize or
// reseed); ExpectedGeneration and the fingerprints bind the step to the state
// that was previewed.
type ExecutionReplaceRequest struct {
	ID                    string      `json:"id"`
	Action                string      `json:"action"`
	ExpectedGeneration    uint64      `json:"expected_generation"`
	CompletionGeneration  uint64      `json:"completion_generation,omitempty"`
	InventoryFingerprint  string      `json:"inventory_fingerprint,omitempty"`
	QuiescenceFingerprint string      `json:"quiescence_fingerprint,omitempty"`
	Reason                string      `json:"reason,omitempty"`
	Actor                 NativeActor `json:"actor"`
	CWD                   string      `json:"cwd"`
	Confirm               bool        `json:"confirm,omitempty"`
}

// ReplacementWorkspaceProcess is one process observed holding a file in the
// workspace being replaced: its PID, command, file descriptor, access mode and path.
type ReplacementWorkspaceProcess struct {
	PID     int
	Command string
	FD      string
	Access  string
	Path    string
}
