package artifactreadability

type Finding struct {
	Code    string `json:"code"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

type Report struct {
	OK       bool      `json:"ok"`
	Critical []Finding `json:"critical"`
	Warnings []Finding `json:"warnings"`
}
