// Package qualitycatalog defines the DTOs of the quality candidate catalog and
// how each candidate's change is verified. It sits in the contract layer.
package qualitycatalog

// Candidate is one ranked quality improvement. Score derives from Impact,
// Feasibility, and Risk; VerifyWith names the external signals that prove it.
type Candidate struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Status      string   `json:"status"`
	Score       float64  `json:"score"`
	Impact      float64  `json:"impact"`
	Feasibility float64  `json:"feasibility"`
	Risk        float64  `json:"risk"`
	VerifyWith  []string `json:"verify_with"`
	Evidence    []string `json:"evidence"`
}
