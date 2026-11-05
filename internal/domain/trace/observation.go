package trace

import tracecontract "issueops/internal/contract/trace"

// Evidence 타입은 finding 원인을 뒷받침하는 증거 하나다. Cause는 원인 분류,
// Code는 증거 코드, Source는 출처다.
type Evidence struct{ Cause, Code, Source string }

// Cluster 타입은 같은 단계(Step)에서 실패가 반복된 횟수 묶음이다.
type Cluster struct {
	Step  string
	Count int
}

// Summary 타입은 self-verify 요약 한 건이다. 실패 단계 수와 첫 실패 단계, 실패
// 분류, 단계별 클러스터, 재실행 명령, 원인 증거를 담는다.
type Summary struct {
	FailedSteps              int
	FailedStep, FailureClass string
	Clusters                 []Cluster
	RerunCommands            []string
	Evidence                 []Evidence
	Cause                    string
}

// Upkeep 타입은 문서 관리(doc upkeep) 이벤트 하나다. Kind와 Summary가 둘 다
// 비어 있으면 이벤트가 없다는 뜻이고, TargetDocs는 갱신 대상 문서 목록이다.
type Upkeep struct {
	Kind, Summary string
	TargetDocs    []string
}

// Document 타입은 trace 입력 JSON 한 건(또는 JSONL 한 줄)이다. self-verify 요약,
// guard 규칙, upkeep 이벤트 중 해당하는 부분만 채워진다.
type Document struct {
	Summary              Summary
	HasSummary, HasGuard bool
	TopFailureClass      string
	TopFailedSteps       int
	GuardRules           []string
	Upkeep               Upkeep
	Event, Step          string
	OK                   bool
}

// Input 타입은 Analyze의 입력이다. 단일 Document 또는 JSONL Lines, 입력이 잘렸는지
// 나타내는 Incomplete, 수집 단계의 Warnings, 호스트 사용량 보고(Usage)를 담는다.
type Input struct {
	Document   *Document
	Lines      []Document
	Incomplete bool
	Warnings   []string
	Usage      *tracecontract.UsageReport
}

// Finding 타입은 반복되는 실패 패턴 하나다. 분류와 원인, 원인 증거, 반복 패턴,
// 제안하는 조정 지점(ProposedKnob), 과적합 위험, 검증 명령을 담는다.
type Finding struct {
	FailureClass, FailureCause                                       string
	FailureCauseEvidence                                             []Evidence
	RecurringPattern, ProposedKnob, OverfitRisk, VerificationCommand string
}

// Analysis 타입은 Analyze의 결과다. 중복을 제거한 Findings와 입력 종류(Types),
// 경고, 입력이 불완전했는지를 담는다.
type Analysis struct {
	Findings        []Finding
	Types, Warnings []string
	Incomplete      bool
}
