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
