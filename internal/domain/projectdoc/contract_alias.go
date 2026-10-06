package projectdoc

import projectdoccontract "issueops/internal/contract/projectdoc"

// 신호와 명령 근거는 계약 DTO다. domain은 같은 capability 계약만 import할 수 있어서
// domain/verifywork가 이 두 타입을 이 패키지 이름으로 받는다.
type (
	ProjectSignals  = projectdoccontract.ProjectSignals
	EvidenceCommand = projectdoccontract.EvidenceCommand
)
