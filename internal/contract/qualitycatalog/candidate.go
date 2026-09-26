package qualitycatalog

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
