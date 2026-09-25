# #514 실행 보고: 바이트 상한 절단이 UTF-8 글자를 가르지 않게 한다

- lifecycle ID: `io-34938e479083`
- 이슈: https://github.com/m16khb-org/issueops/issues/514
- 브랜치: `514-utf8-safe-truncation` (base `main` @ `92bbbddabb9bbee9c7a1e050fc6fe061d7d45201`)
- canonical worktree: `/Users/habin/workspace/issueops.worktrees/514-utf8-safe-truncation`
- 실행 holder: direct, generation 2 (claude, claude-opus-5-5 high)
- 계획: `.issueops/issues/514/artifact/plan.md` (3차 봉인)
- 게이트 원장: `.issueops/issues/514/gates.md`
- 상태: 문서 단계 반영. 검증 단계 결과는 뒤에 덧붙인다.

## 무엇을 바꿨나

- `internal/domain/policy/text_bound.go`에 순수 함수 4개를 더했다. `TruncateBytes`(앞에서 자름), `TailBytes`(뒤에서 자름), `TruncateRunes`(글자 수로 자름), `TrimIncompleteRune`(끊긴 마지막 글자를 버림). 표준 라이브러리 `unicode/utf8`만 쓴다.
- 사람이 보거나 record·게이트 원장에 남는 문자열을 바이트로 자르던 20개 지점(18개 파일)이 이 함수를 쓰게 했다. 상한 값, 접미사·표지, `len(x) > limit` 비교, 레드액션 순서, 호출부는 그대로다.
- stderr 캡처 버퍼 두 곳(`providerutil.boundedBuffer`, `remoteverify.remoteVerifyBuffer`)은 상한에서 잘렸을 때만 `String()`이 끊긴 마지막 글자를 버린다. `Write`와 `truncated` 판정, stdout 반환 바이트는 그대로다.
- `webfetch.TruncateContent`는 스키마 설명대로 `maxChars`를 글자 수로 센다.
- `commandstep.TailWithBudget`은 꼬리를 글자 경계에서 자르고 표지의 `omitted_bytes`를 실제로 뺀 바이트 수로 계산한다.

## AI slop 정리

- 정리 범위: 이번 diff(소스 18개, 새 파일 22개).
- 제거한 중복(duplication): `TailWithBudget`의 표지 `Sprintf`가 루프 안과 고정점 분기에 두 번 있었다. 루프 안에서 꼬리를 먼저 구하고 그 길이로 표지를 한 번만 만든다. ASCII 입력에서는 꼬리 길이가 예산과 같아 결과가 바뀌지 않는다.
- 동작 보존 확인: 기존 구현을 복사한 임시 비교 테스트로 ASCII 입력 449,330개 조합(길이 0~12000, 상한 -1~260)의 결과·잘림 여부·원래 길이가 같음을 확인했다. 여러 바이트 입력에서는 결과 길이가 상한 이하였고, 상한 64 이상에서 유효한 UTF-8이었다. 비교 파일은 확인 뒤 지웠다.
- 의도적으로 남긴 것: exported 함수 4개의 doc 주석과 `TruncateContent`의 계약 주석, 테스트 입력의 바이트 계산 주석. 상한이 글자 중간에 걸리는 이유를 설명하므로 지우면 테스트 의도를 알 수 없다.
- 범위 밖 발견: `cmd/issueops/issueopscli/remotecmd/remote.go:262-316`의 두 블록이 dupl에 걸린다. 이번 변경이 건드리지 않은 기존 코드이고 #513 작업 구간이라 손대지 않았다.
- 지표(정리 전 → 후, Go 파일만, 줄 기반 근사):

  | 지표 | 전 | 후 |
  |---|---|---|
  | SNR(추가 비공백 573줄 중 주석 13줄) | 0.977 | 0.977 |
  | 새 파일의 dupl 적중(golangci-lint `--enable-only dupl`) | 0 | 0 |
  | `TailWithBudget` 표지 형식 문자열 | 2곳 | 1곳 |
  | 이번 diff가 바꾼 함수의 분기 수 | 기존과 같음(`TruncateContent`만 2 → 1) | 같음 |

## 성공 기준과 관측

| 기준 | 판정 | 근거 |
|---|---|---|
| 20개 지점이 글자 중간 상한에서도 유효한 UTF-8을 낸다 | PASS | `go test <17개 패키지> -run UTF8Safe -count=1 -v` → 24개 `--- PASS` |
| 결과 길이가 기존 상한 + 접미사·표지 길이를 넘지 않는다 | PASS | 같은 테스트의 길이 검사 |
| ASCII 결과와 기존 테스트 유지 | PASS | G3 `unchanged`, G4 `ALL_PASS` |
| 한글 stderr 진단이 유효한 UTF-8 | PASS | `TestBoundedCommandStderrUTF8Safe`, `TestRemoteVerifyStderrUTF8Safe` |
| DryRunPreview의 JSON 인코딩에 U+FFFD가 없다 | PASS | `TestDryRunPreviewUTF8Safe` |
| `TruncateContent`가 글자 수로 자른다 | PASS | `TestTruncateContentUTF8SafeCountsCharacters` |
| `go test ./...`, `go vet ./...` 통과 | PASS | 원장 G8·G10(종료 코드 0) |

## TDD 기록

- T1 RED: `go test ./internal/domain/policy -run UTF8Safe -count=1 -v` → `undefined: TruncateBytes` 등 컴파일 실패, 종료 코드 1.
- T1 GREEN: 같은 명령 → 테스트 4개 PASS. `go test ./internal/domain/policy ./internal/architecture -count=1` → ok.
- T2 RED: 새 테스트 20개를 먼저 추가하고 기존 코드에서 17개 패키지를 `-run UTF8Safe`로 실행 → 20개 `--- FAIL`, T1 테스트 4개 PASS.
- T2 GREEN: 20개 지점을 고친 뒤 같은 명령 → 24개 `--- PASS`. `go build ./...` 통과, `go test ./internal/architecture -count=1` → ok.
- 게이트 부분 실행(원장 CHECK 그대로): G3 `unchanged`, G4 `ALL_PASS`, G5 `clean`, G6 종료 코드 0, G7 `clean`, G9 종료 코드 0.

## 게이트 원장 실행

- 1차 run(2026-09-24 05:50Z 종료, 1시간 52분): 12개 충족. G10·G11은 CHECK 한도 900초를 넘겨 `check timed out`으로 미충족이었다. 실패한 테스트는 없었다. 같은 시간대 머신 load average가 100~290이었다(코어 8개).
- 한도를 늘린 재시도는 명령 정책이 `timeout_exceeds_15m`으로 거부했다. 한도는 900초로 유지했다.
- 사용자가 상주 프로세스를 줄인 뒤 부하가 가라앉은 상태(1분 평균 8.3)에서 같은 원장으로 다시 실행했다. `gates check`는 미충족 게이트만 다시 실행하므로 G10·G11만 실행됐고 둘 다 충족됐다(26분). 1차와 2차 사이에 코드 변경은 없다.
- 최종 상태: `issueops gates status` → 14개 충족, 미충족 0.
- G14 사전 점검: 원장 run 전 단독 실행은 self-verify 안의 race 테스트가 10분 한도에 걸려 실패했다(같은 부하 원인). 이 실행이 남긴 고아 테스트 프로세스(pid 84842)를 종료했다. 원장 1차 run에서는 G14가 통과했다.

## base 앞섬

- 작업 중 `main`이 `844e69c8`(#513 머지)로 앞섰다. `issueops execution sync-base --preview` → `merge_needed: true`, 충돌 파일 목록 없음.
- apply하지 않았다. 추적 중인 미커밋 변경이 있어 apply는 `worktree_clean`으로 거부되고, 적용하면 변경 집합이 봉인 base 기준이라 #513 diff 전체가 이 사이클의 리뷰 대상에 들어온다. 머지는 PR 병합 때 provider가 한다(`issueops-verify` 4절).
- 충돌 예측: 작업 트리 내용을 임시 인덱스로 만든 커밋과 `origin/main`의 `git merge-tree --write-tree` → 종료 코드 0, 충돌 없음. 실제 인덱스와 작업 트리는 바꾸지 않았다.

## 문서 반영

- 대조한 문서: `AGENTS.md`, `.issueops/AGENT_WORKFLOW.md`, `.issueops/TESTING.md`(route 결과), `.issueops/CAUTIONS.md` 색인, `.issueops/cautions/2026-09-20-native-host-probe-evidence-and-bounded-output.md`, 계획의 `## 적용되는 결정과 주의사항` 표의 문서들.
- 문서 → 구현: 의존 방향(domain은 표준 라이브러리만 추가), 레드액션 뒤 절단 순서, 상한 값 유지를 어기지 않았다. `go test ./internal/architecture`가 G10 안에서 통과했다.
- 구현 → 문서: caution 기록 두 개를 append했다.
  - `.issueops/cautions/2026-09-25-utf-8-safe-byte-bounded-truncation.md`: 계획 T3의 재발 방지 항목.
  - `.issueops/cautions/2026-09-25-gate-check-15-minute-cap-under-host-load.md`: 계획 때 몰랐던 함정. CHECK 한도 900초 정책과 미충족 게이트만 다시 실행하는 동작.
- golden: dated 기록은 `docs_index`에 들어가지 않아 `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1`이 재생성 없이 통과했다. `./bin/issueops docs --json` 종료 코드 0.

## Side effect

- 파일: 소스 18개 수정, 새 파일 22개(`text_bound.go`와 테스트 21개), caution 기록 2개, 게이트 원장과 이 보고서.
- 로컬 프로세스: G14 사전 점검이 남긴 고아 테스트 프로세스 하나를 종료했다. 그 밖에 남긴 프로세스는 없다.
- 원격: 없음. provider로 보내는 본문은 자르지 않는다.
- durable state: 새로 쓰는 `Execution.Failure.Message`, preparation 실패 메시지, 이슈 생성 실패 진단, 게이트 원장 EVIDENCE가 유효한 UTF-8이 된다. 이미 저장된 값과 record 스키마는 바뀌지 않는다.
- CLI·MCP 출력: ASCII는 바이트 단위로 같다. 여러 바이트 글자가 상한에 걸릴 때만 최대 3바이트(`ellipsizeMiddle`은 최대 6바이트) 짧아진다. `web_fetch_resilient`의 `max_chars`는 글자 수가 되어 한글 응답의 바이트가 최대 3배까지 늘 수 있다.

## 성능

- hot path가 아니다. 추가 비용은 `TruncateBytes`·`TailBytes`·`TrimIncompleteRune`에서 최대 3바이트 검사(O(1)), `TruncateRunes`에서 O(maxRunes)다. `max_chars` 기본값 0에서는 `TruncateRunes`가 실행되지 않는다. 측정할 병목이 없어 벤치마크는 두지 않았다(계획의 `## 성능 영향`).

## 남은 위험

- base가 `844e69c8`로 앞서 있다. 충돌 예측은 깨끗하지만 PR 병합 전까지 이 브랜치의 로컬 검증은 base `92bbbdda` 기준이다.

```text
Success criteria: intent 성공 기준 7개, 게이트 G1~G14
Evidence artifact: .issueops/issues/514/gates.md, 이 보고서
Cleanup receipt: none spawned
Verification mode: full loop
Skipped checks: none. G10·G11은 부하로 한 번 시간 초과된 뒤 같은 코드에서 다시 실행해 채웠다
```
