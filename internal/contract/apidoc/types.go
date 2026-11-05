// Package apidoc holds the result DTOs of the API documentation checks: the
// deterministic static check, the agent review, and their combined verdict.
// It is a contract package with no behavior.
package apidoc

// Violation is one static API documentation problem: the file and line it was
// found at, a machine-readable code, and a human-readable message.
type Violation struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// StaticResult is the outcome of the static API documentation check. Skipped
// with a Reason means no files were eligible rather than a failure.
type StaticResult struct {
	OK         bool        `json:"ok"`
	Summary    string      `json:"summary"`
	Files      []string    `json:"files"`
	Violations []Violation `json:"violations"`
	Skipped    bool        `json:"skipped,omitempty"`
	Reason     string      `json:"reason,omitempty"`
}

// ReviewFinding is one reviewer finding. Line is nil when the finding applies
// to the whole file.
type ReviewFinding struct {
	File     string `json:"file"`
	Line     *int   `json:"line"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// ReviewResult is the agent review outcome. When no verdict exists yet it
// carries the Prompt and Schema for the host agent to answer, and ResultFile
// names where a recorded review result was read from.
type ReviewResult struct {
	OK         bool            `json:"ok"`
	Verdict    string          `json:"verdict"`
	Summary    string          `json:"summary"`
	Findings   []ReviewFinding `json:"findings"`
	Files      []string        `json:"files"`
	Skipped    bool            `json:"skipped,omitempty"`
	Reason     string          `json:"reason,omitempty"`
	Prompt     string          `json:"prompt,omitempty"`
	Schema     map[string]any  `json:"schema,omitempty"`
	ResultFile string          `json:"result_file,omitempty"`
}

// CheckResult combines the static check and the review into one verdict; Reason
// explains a failing or skipped combined result.
type CheckResult struct {
	OK     bool         `json:"ok"`
	Static StaticResult `json:"static"`
	Review ReviewResult `json:"review"`
	Reason string       `json:"reason,omitempty"`
}
