package issueopslease

type ReseedReceipt struct {
	Execution      Execution `json:"execution"`
	ClaimTokenPath string    `json:"claim_token_path,omitempty"`
}
