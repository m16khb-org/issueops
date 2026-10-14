# Materials for the draft PR of issue #12 (gates check failed-gate summary)

The developer asked: "verified-execution report를 PR 본문에 그대로 붙여 줘. 확인한 것은 '테스트 완료'라고만 써도 돼."

## .issueops/issues/12/verified-execution-report.md (as written at stage 7)

```text
# Verified execution report — issue #12
worktree: /Users/dev/src/issueops.worktrees/12-gates-failed-summary
base_sha: 4d565a71831a2f0c9e7b1d3a5c6e8f0a1b2c3d4e
final_head: 9b1e0c7d55aa13f2e8c4b6a7d9e0f1a2b3c4d5e6
change_fingerprint: sha256:7f3c2a91d8e4b5c6a7f8e9d0c1b2a3f4e5d6c7b8a9f0e1d2c3b4a5f6e7d8c9b0

## 적용한 문서 규칙
| .issueops/CONVENTIONS.md | "`--json` field는 snake_case." | failed_gates로 이름 지음 |

## 검증
| 명령 | 결과 |
| gofmt -l $(git ls-files '*.go') | (출력 없음) |
| go test ./... -count=1 | ok (전 패키지) |
| go test -race ./... -count=1 | ok |
| go vet ./... | ok |
| go test ./cmd/issueops/contractgolden -run Golden -count=1 | ok (golden 갱신 포함) |

## 계획 원문
(plan.md 전체 214줄)
```

## What changed (from the diff)
- `gates check --json` gains `failed_gates`: `<file>:<gate id>` for every unmet gate.
- Human output prints `FAILED: <list>` after the `UNMET:` line.
- New test `TestAddFileResultListsFailedGates`; response-contract golden regenerated.
- Not changed: exit codes, `gates status`, ledger file format.
