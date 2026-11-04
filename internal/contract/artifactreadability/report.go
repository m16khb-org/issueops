// Package artifactreadability holds the contract DTOs of the artifact
// readability check: the findings and the report returned to callers.
package artifactreadability

// Finding is one readability problem found in a generated artifact. Code is a
// stable machine-readable identifier; Line is the 1-based line, or 0 when the
// problem is not tied to a line.
type Finding struct {
	Code    string `json:"code"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

// Report is the readability check result for one artifact. Critical findings
// make the artifact unacceptable; Warnings are advisory. OK is true only when
// there are no critical findings.
type Report struct {
	OK       bool      `json:"ok"`
	Critical []Finding `json:"critical"`
	Warnings []Finding `json:"warnings"`
}
