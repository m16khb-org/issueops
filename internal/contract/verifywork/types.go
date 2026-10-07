package verifywork

import (
	guardcontract "issueops/internal/contract/guard"
	policydomain "issueops/internal/contract/policy"
	preflightcontract "issueops/internal/contract/preflight"
)

type Result struct {
	OK                bool                              `json:"ok"`
	Kind              string                            `json:"kind"`
	Repo              string                            `json:"repo"`
	GitStatus         string                            `json:"git_status,omitempty"`
	Preflight         preflightcontract.PreflightResult `json:"preflight"`
	Guard             guardcontract.GuardCheckResult    `json:"guard"`
	Command           *policydomain.CommandRunResult    `json:"command,omitempty"`
	EvidenceMatrix    []EvidenceItem                    `json:"evidence_matrix"`
	SuggestedCommands []SuggestedCommand                `json:"suggested_commands"`
	Warnings          []string                          `json:"warnings"`
}

type EvidenceItem struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
	Command string `json:"command,omitempty"`
}

type SuggestedCommand struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
	Reason  string   `json:"reason"`
}
