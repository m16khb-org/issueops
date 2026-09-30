# Gates: 523

- [x] G1: 최신 적격 요약과 오류 처리가 일관된다
  CHECK: go test ./internal/domain/status ./internal/application/status ./internal/application/selfaugment ./internal/domain/selfaugment ./cmd/issueops/statuscli -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/domain/selfaugment	0.328s | ok  	issueops/cmd/issueops/statuscli	7.283s
- [x] G2: DDD 의존 방향을 지킨다
  CHECK: go test ./internal/architecture -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/architecture	7.119s
- [x] G3: CLI 응답 계약이 유지된다
  CHECK: go test ./cmd/issueops/contractgolden -run Golden -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/cmd/issueops/contractgolden	0.278s
