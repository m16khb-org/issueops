package issueopsreview

// CompatibilityReview 타입은 변경의 하위 호환성, 부작용, 롤백 계획을 검토한
// 기록이다. Blockers에는 승인을 막는 항목이, Approved에는 승인 여부가 담긴다.
type CompatibilityReview struct {
	BackwardCompatibility []string `json:"backward_compatibility"`
	SideEffects           []string `json:"side_effects"`
	RollbackPlan          string   `json:"rollback_plan"`
	Verification          []string `json:"verification"`
	Blockers              []string `json:"blockers,omitempty"`
	Approved              bool     `json:"approved"`
	ReviewedAt            string   `json:"reviewed_at"`
}

// CompatibilityReviewRequest 타입은 호환성 검토를 기록하려는 호출자가 넘기는
// 입력이다. ReviewedAt은 기록 시점에 채워지므로 포함하지 않는다.
type CompatibilityReviewRequest struct {
	BackwardCompatibility []string
	SideEffects           []string
	RollbackPlan          string
	Verification          []string
	Blockers              []string
	Approved              bool
}
