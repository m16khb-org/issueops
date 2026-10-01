# Gates: 533

- [x] G1: 부분 실패와 중복 생성 회귀
  CHECK: go test ./internal/application/issueopsremote ./internal/adapter/provider/github ./internal/adapter/provider/gitlab -count=1
  EXPECT: /^ok\s/m
  EVIDENCE: ok  	issueops/internal/adapter/provider/github	7.087s | ok  	issueops/internal/adapter/provider/gitlab	8.464s
- [x] G2: identity와 공유 record 및 CLI
  CHECK: go test ./internal/domain/issueops ./internal/contract/issueops ./cmd/issueops/issueopscli/... ./cmd/issueops/issueopsapp -count=1
  EXPECT: /^ok\s/m
  EVIDENCE: ok  	issueops/cmd/issueops/issueopscli/remotecmd	32.382s | ok  	issueops/cmd/issueops/issueopsapp	67.090s
- [x] G3: architecture와 golden 계약
  CHECK: go test ./internal/architecture ./cmd/issueops/contractgolden -count=1
  EXPECT: /^ok\s/m
  EVIDENCE: ok  	issueops/internal/architecture	8.882s | ok  	issueops/cmd/issueops/contractgolden	0.289s
