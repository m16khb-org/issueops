package issueopsreview

type CompatibilityReview struct {
	BackwardCompatibility []string `json:"backward_compatibility"`
	SideEffects           []string `json:"side_effects"`
	RollbackPlan          string   `json:"rollback_plan"`
	Verification          []string `json:"verification"`
	Blockers              []string `json:"blockers,omitempty"`
	Approved              bool     `json:"approved"`
	ReviewedAt            string   `json:"reviewed_at"`
}

type CompatibilityReviewRequest struct {
	BackwardCompatibility []string
	SideEffects           []string
	RollbackPlan          string
	Verification          []string
	Blockers              []string
	Approved              bool
}
