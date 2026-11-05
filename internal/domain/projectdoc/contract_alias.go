package projectdoc

import projectdoccontract "issueops/internal/contract/projectdoc"

// domain은 같은 capability 계약만 import할 수 있어서 domain/verifywork가 이 두
// 타입을 이 패키지 이름으로 받는다.
type (
	// ProjectSignals 타입은 저장소 스캔이 모은 문서 생성 근거를 담는 계약 DTO다.
	ProjectSignals = projectdoccontract.ProjectSignals
	// EvidenceCommand 타입은 근거와 함께 추정한 명령 한 건을 담는 계약 DTO다.
	EvidenceCommand = projectdoccontract.EvidenceCommand
)
