package basiccli

import (
	issueopscontract "issueops/internal/contract/issueops"
	tracecontract "issueops/internal/contract/trace"
)

// 이 연산은 실제 I/O를 수행한다. 구현은 composition root가 설치한다.
var (
	TraceAnalyze                func(req tracecontract.TraceAnalyzeRequest) (tracecontract.TraceAnalyzeResult, error)
	TraceHandoffDeliveryObserve func(issueopscontract.IssueOpsHandoffDeliveryObservation) (tracecontract.HandoffDeliveryObserveResult, error)
)
