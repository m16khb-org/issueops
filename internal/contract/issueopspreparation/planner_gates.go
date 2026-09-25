package issueopspreparation

// PlannerGate는 owner가 스스로 채울 수 없는 planner 소유 전제 하나다.
type PlannerGate struct {
	// Key는 owner가 나중에 받게 될 missing key와 같은 이름이다. 두 곳이 다른
	// 이름을 쓰면 사용자가 같은 사실을 두 번 조사하게 된다.
	Key string
	// Command는 coordinator가 그것을 기록하는 정확한 명령이다. 진단만 있고
	// 명령이 없으면 사용자는 무엇을 실행할지 추측하게 된다.
	Command string
}

// PlannerEvidence is the decoded planner-owned input for prepare policy.
type PlannerEvidence struct {
	Intent         PlannerIntentEvidence
	DesignReview   PlannerDesignEvidence
	DevilsAdvocate PlannerDevilsAdvocateEvidence
}

type PlannerIntentEvidence struct {
	RawRequest        string   `json:"raw_request"`
	InterpretedIntent string   `json:"interpreted_intent"`
	SuccessCriteria   []string `json:"success_criteria"`
}

type PlannerDesignEvidence struct {
	ProblemSummary string   `json:"problem_summary"`
	ProposedDesign string   `json:"proposed_design"`
	Verification   []string `json:"verification"`
	Approved       bool     `json:"approved"`
}

type PlannerDevilsAdvocateEvidence struct {
	Verdict            string `json:"verdict"`
	Waived             bool   `json:"waived"`
	ReviewerPattern    string `json:"reviewer_pattern"`
	ReviewedPlanDigest string `json:"reviewed_plan_digest"`
	RecordedAt         string `json:"recorded_at"`
}

// PlannerGateKeys는 진단 문구에 넣을 키 목록이다.
func PlannerGateKeys(gates []PlannerGate) []string {
	keys := make([]string, 0, len(gates))
	for _, gate := range gates {
		keys = append(keys, gate.Key)
	}
	return keys
}
