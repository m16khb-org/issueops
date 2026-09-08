package issueopslease

type ReseedReceipt struct {
	Execution           Execution `json:"execution"`
	ClaimTokenPath      string    `json:"claim_token_path,omitempty"`
	IssueBodySHA256     string    `json:"issue_body_sha256,omitempty"`
	ContextPacketPath   string    `json:"context_packet_path,omitempty"`
	ContextPacketSHA256 string    `json:"context_packet_sha256,omitempty"`
	OwnerPromptPath     string    `json:"owner_prompt_path,omitempty"`
	OwnerPromptSHA256   string    `json:"owner_prompt_sha256,omitempty"`
}
