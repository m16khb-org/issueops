# Gates: 542

- [x] G1: 감사표 정상과 오류 구분
  EVIDENCE: ok  	issueops/internal/adapter/outbound/quality	0.393s
  CHECK: go test ./internal/adapter/outbound/quality -run TestCollectAuditItems -count=1
  EXPECT: ok
- [x] G2: 품질 상태로 오류 전달
  EVIDENCE: ok  	issueops/internal/domain/quality	0.320s | ok  	issueops/cmd/issueops/qualitycli	2.878s
  CHECK: go test ./internal/application/quality ./internal/domain/quality ./cmd/issueops/qualitycli -count=1
  EXPECT: ok
- [x] G3: 품질 패키지 race
  EVIDENCE: ok  	issueops/internal/domain/quality	1.116s | ok  	issueops/cmd/issueops/qualitycli	3.906s
  CHECK: go test -race ./internal/adapter/outbound/quality ./internal/application/quality ./internal/domain/quality ./cmd/issueops/qualitycli -count=1
  EXPECT: ok
- [x] G4: 공개 composition 유지
  EVIDENCE: ok  	issueops/cmd/issueops/issueopsapp	2.259s
  CHECK: go test ./cmd/issueops/issueopsapp -run TestQuality -count=1
  EXPECT: ok
- [x] G5: MCP catalog 불변
  EVIDENCE: ok  	issueops/cmd/issueops/contractgolden	0.282s
  CHECK: go test ./cmd/issueops/contractgolden -run ^TestMCPToolsGolden$ -count=1
  EXPECT: ok
- [x] G6: response contract 불변
  EVIDENCE: ok  	issueops/cmd/issueops/issueopsapp	5.998s
  CHECK: go test ./cmd/issueops/issueopsapp -run ^TestResponseContractsGolden$ -count=1
  EXPECT: ok
