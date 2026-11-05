package selfverify

import "issueops/internal/contract/failurecause"

// StepResult records one executed self-verification step: its label and
// command, outcome, whether a previous result was reused, duration, and the
// captured output with its original byte sizes and truncation flags.
type StepResult struct {
	Label           string                  `json:"label"`
	Command         string                  `json:"command,omitempty"`
	OK              bool                    `json:"ok"`
	Reused          bool                    `json:"reused"`
	DurationMS      int64                   `json:"duration_ms"`
	Stdout          string                  `json:"stdout,omitempty"`
	Stderr          string                  `json:"stderr,omitempty"`
	StdoutBytes     int                     `json:"stdout_bytes,omitempty"`
	StderrBytes     int                     `json:"stderr_bytes,omitempty"`
	StdoutTruncated bool                    `json:"stdout_truncated,omitempty"`
	StderrTruncated bool                    `json:"stderr_truncated,omitempty"`
	FailureEvidence []failurecause.Evidence `json:"failure_evidence,omitempty"`
	Error           string                  `json:"error,omitempty"`
}
