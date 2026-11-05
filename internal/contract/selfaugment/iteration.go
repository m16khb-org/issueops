package selfaugment

import selfverify "issueops/internal/contract/selfverify"

// Iteration 타입은 자기 검증 루프의 한 번의 반복 실행이다.
// 반복 번호, 그 반복에 쓰인 시드, 단계별 결과를 순서대로 담는다.
type Iteration struct {
	Iteration int                     `json:"iteration"`
	Seed      int64                   `json:"seed"`
	Steps     []selfverify.StepResult `json:"steps"`
}
