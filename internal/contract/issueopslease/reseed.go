package issueopslease

// ReseedReceipt 타입은 execution을 다시 시드한 결과다. 새 execution과, 새 claim token을 쓴 파일 경로(있을 때)를 담는다.
type ReseedReceipt struct {
	Execution      Execution `json:"execution"`
	ClaimTokenPath string    `json:"claim_token_path,omitempty"`
}
