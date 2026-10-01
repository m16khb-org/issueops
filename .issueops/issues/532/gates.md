# Gates: 532

- [x] G1: 프로세스와 출력 경계 회귀
  EVIDENCE: ok  	issueops/internal/application/policy	0.437s | ok  	issueops/internal/domain/policy	0.493s
  CHECK: go test ./internal/adapter/policy ./internal/application/policy ./internal/domain/policy -count=1
  EXPECT: ok
- [x] G2: 정책 실행 경쟁 검사
  EVIDENCE: ok  	issueops/internal/adapter/policy	9.829s | ok  	issueops/internal/application/policy	1.590s
  CHECK: go test -race ./internal/adapter/policy ./internal/application/policy -count=1
  EXPECT: ok
- [x] G3: 계층 경계
  EVIDENCE: ok  	issueops/internal/architecture	7.525s
  CHECK: go test ./internal/architecture -count=1
  EXPECT: ok
- [x] G4: 응답 golden
  EVIDENCE: ok  	issueops/cmd/issueops/contractgolden	0.316s | ok  	issueops/cmd/issueops/issueopsapp	7.413s
  CHECK: go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1
  EXPECT: ok
