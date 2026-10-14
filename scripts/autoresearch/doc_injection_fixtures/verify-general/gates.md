# Gates: gates check failed-gate summary

- [x] G1: gates check --json lists every unmet gate | CHECK: go test ./internal/domain/gates -run TestSummarizeFileAndAddFileResultAggregateTotals -count=1 | EXPECT: ok
  - EVIDENCE: ok  	issueops/internal/domain/gates	0.31s
- [x] G2: human output prints a FAILED line when gates are unmet | CHECK: go build ./cmd/issueops | EXPECT:
  - EVIDENCE: (empty output, exit 0)
