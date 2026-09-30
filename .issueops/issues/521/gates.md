# Gates: 521

- [x] G1: 제품 파일 선정과 오류 경계 보존
  CHECK: go test ./internal/adapter/outbound/quality -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/adapter/outbound/quality	1.427s
- [x] G2: 실제 quality 명령과 wiring 계약 유지
  CHECK: go test ./cmd/issueops/qualitycli ./cmd/issueops/issueopsapp -run Quality -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/cmd/issueops/qualitycli	0.486s | ok  	issueops/cmd/issueops/issueopsapp	2.056s
- [x] G3: 계층 의존 방향 유지
  CHECK: go test ./internal/architecture -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/architecture	7.406s
