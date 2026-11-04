// Package failurecause 패키지는 실패 원인 분류의 DTO를 소유한다.
//
// 분류 자체는 진단 문자열 redaction이 필요해 adapter에 남지만, 결과를 읽고
// 전달하는 쪽은 분류 구현을 알 필요가 없다.
package failurecause

// Cause 타입은 실패를 일으킨 원인의 범주를 나타내는 열거 문자열이다.
type Cause string

// Cause의 허용 값이다. None은 실패가 없음을, Unknown은 어느 범주로도 분류하지 못했음을 뜻한다.
const (
	None               Cause = "none"
	Model              Cause = "model"
	HarnessEnvironment Cause = "harness_environment"
	Transport          Cause = "transport"
	ContractInput      Cause = "contract_input"
	Unknown            Cause = "unknown"
)

// Evidence 타입은 원인 판정의 근거 한 건이다. Code는 진단 코드, Source는 그 코드를 관측한 출처다.
type Evidence struct {
	Cause  Cause  `json:"cause"`
	Code   string `json:"code"`
	Source string `json:"source"`
}

// Result 타입은 실패 원인 분류의 결과다. Cause는 최종 판정, Reason은 판정 사유, Evidence는 판정 근거 목록이다.
type Result struct {
	Cause    Cause      `json:"cause"`
	Reason   string     `json:"reason"`
	Evidence []Evidence `json:"evidence"`
}
