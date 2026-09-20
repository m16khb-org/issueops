package issueopslease

type ResumeArtifacts struct {
	ClaimTokenPath      string `json:"claim_token_path"`
	IssueBodySHA256     string `json:"issue_body_sha256"`
	ContextPacketPath   string `json:"context_packet_path"`
	ContextPacketSHA256 string `json:"context_packet_sha256"`
	OwnerPromptPath     string `json:"owner_prompt_path"`
	OwnerPromptSHA256   string `json:"owner_prompt_sha256"`
}

type ResumeReceipt struct {
	Execution Execution       `json:"execution"`
	Artifacts ResumeArtifacts `json:"artifacts"`
}

type ResumeStageReceipt struct {
	TerminalPTYID  string             `json:"terminal_pty_id,omitempty"`
	TerminalHandle string             `json:"terminal_handle,omitempty"`
	RunID          string             `json:"run_id,omitempty"`
	RunBound       bool               `json:"run_bound,omitempty"`
	TaskID         string             `json:"task_id,omitempty"`
	DispatchID     string             `json:"dispatch_id,omitempty"`
	RequestID      string             `json:"request_id,omitempty"`
	PromptReceipt  *OrcaPromptReceipt `json:"prompt_receipt,omitempty"`
}

type OrcaPromptReceipt struct {
	RequestID               string   `json:"request_id"`
	Stages                  []string `json:"stages,omitempty"`
	Provider                string   `json:"provider,omitempty"`
	Observation             string   `json:"observation,omitempty"`
	ProcessIncarnation      string   `json:"process_incarnation,omitempty"`
	Generation              uint64   `json:"generation,omitempty"`
	BaselineWorkingSequence *uint64  `json:"baseline_working_sequence,omitempty"`
}

type ResumeStageInventory struct {
	Candidates        []ResumeStageReceipt `json:"candidates"`
	AuthoritativeZero bool                 `json:"authoritative_zero"`
	ExactReplay       bool                 `json:"exact_replay,omitempty"`
}
