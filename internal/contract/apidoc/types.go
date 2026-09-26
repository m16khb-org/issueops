package apidoc

type Violation struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type StaticResult struct {
	OK         bool        `json:"ok"`
	Summary    string      `json:"summary"`
	Files      []string    `json:"files"`
	Violations []Violation `json:"violations"`
	Skipped    bool        `json:"skipped,omitempty"`
	Reason     string      `json:"reason,omitempty"`
}

type ReviewFinding struct {
	File     string `json:"file"`
	Line     *int   `json:"line"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

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
