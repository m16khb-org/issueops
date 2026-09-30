# Gates: 527

- [x] G1: 공개 경로와 원본 불변성
  CHECK: go test ./internal/domain/issueops ./internal/application/issueopsremote ./internal/adapter/issueops -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/application/issueopsremote	0.336s | ok  	issueops/internal/adapter/issueops	165.411s
- [x] G2: 실제 공개 추적 파일 검사
  CHECK: python3 -m unittest discover -s scripts -p '*_test.py'
  EXPECT: OK
  EVIDENCE: Ran 53 tests in 74.655s | OK (skipped=1)
