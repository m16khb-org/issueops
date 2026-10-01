# #532 정책 실행기의 프로세스 종료와 출력 상한

## 목적과 승인 범위

Cycle `io-7426b49dc042`, issue https://github.com/m16khb-org/issueops/issues/532.
Base main `b917fc960b72bca3e616df1162c6b89d40a517ba`, branch `532-policy-execution-bounds`.
사용자는 다섯 개선 작업의 병렬 IssueOps 인계부터 main 병합, cleanup, io-update까지 승인했다. 이 native owner의 종료점은 검증된 draft PR 게시, 최신 push/PR CI 성공, execution complete와 release다. root가 최종 병합·cleanup·설치 갱신을 맡는다. 이전 intent의 조사 전용 제한은 최신 scope decision이 대체한다.
준비 세션은 direct execution prepare로 canonical worktree를 만들고 실제 holder를 release한 뒤 Orca의 새 Codex 세션(gpt-6.1-sol/high)에 인계한다. 구현은 해당 워크트리에서만 수행한다. 다른 네 작업을 수정하거나 전역 설치하지 않는다.

## Context

`internal/adapter/policy/policy_run.go:51-68`는 CommandContext와 무제한 bytes.Buffer를 사용한다. `internal/application/policy/runner.go:81-82,103-109`는 전체 출력 수집 뒤 redaction 및 32KiB 절단을 수행한다. 감사 증거는 source checkout `.issueops/evidence/audit-20260930/runtime-evidence/`에 있다. 허용 Python 부모/자식 probe는 100ms timeout에서도 1112ms 뒤 반환했고 자식 marker가 남았다. 8MiB 출력은 8MiB 보관 및 약40MiB TotalAlloc 증가를 관측했다. RSS/OOM으로 해석하지 않는다.

### Gap Analysis

timeout은 직접 프로세스 종료만으로 증명되지 않는다. 부모가 일찍 끝나고 자식이 pipe를 보유하는 경우, timeout 직전 종료 경쟁, 양쪽 출력, 실패한 start와 정상/nonzero 종료를 구분한다. 출력의 임의 byte prefix를 자르면 비밀값 패턴과 UTF-8이 훼손될 수 있으므로 상한 경계의 보수적 tail 처리와 실제 redaction 회귀를 포함한다. hostprobe 재사용 때문에 서로 다른 capability adapter import를 추가하지 않는다.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 제2장/제3장: cwd, timeout, env, secret redaction과 순수 domain/application 경계를 유지한다.
- `.issueops/ARCHITECTURE.md` 의존 방향 불변식 및 `.issueops/CONVENTIONS.md`: process IO는 adapter에 두고 cross-capability concrete import를 늘리지 않는다.
- `.issueops/ADR.md`와 `adr/decisions/2026-08-08-dependency-ratchet-capability-boundary.md`: hostprobe를 통째로 import하는 일반 실행기 통합은 범위 밖이다.
- `.issueops/CAUTIONS.md`, `cautions/security.md` 위험한 shell 실행/Secret leakage: 기존 argv 평가, env allowlist, stdout/stderr redaction을 보존하며 fixture에는 가짜 secret만 사용한다.
- `cautions/runtime.md` Worker lifecycle: 종료 signal만으로 자손 종료를 보고하지 않는다. 테스트 프로세스와 임시 파일은 종료·회수한다.
- `.issueops/TESTING.md`, `testing/concurrency-and-race.md`: 실제 프로세스와 pipe 증거를 남기고 경쟁을 race로 확인한다. 좁은 process fixture는 handshake와 넉넉한 bounded deadline으로 환경 flake를 줄인다.

## 재사용하는 기존 구현

- policy `Service.run`, `CommandEnvironment`, `CommandTimeout`, `RedactFreeform`, `TruncateBytes`와 기존 policy tests를 유지한다.
- `internal/adapter/hostprobe/runner.go:66-133`의 bounded writer 및 process-group 종료 패턴을 참고한다. bytes.Buffer를 embedding해 ReadFrom이 Write를 우회하지 않도록 주의한다. hostprobe 전체 runner/interface를 policy에 가져오지 않는다.
- 기존 policy 테스트의 허용 request와 Go helper-process 구조를 재사용한다. helper가 없으면 이 경계 테스트에 필요한 최소 helper만 둔다.

## 구현과 검증

1. 실제 subprocess 회귀 테스트부터 추가하고 현재 구현에서 실패를 확인한다. 부모·자식 handshake 후 timeout, 부모 조기 종료+상속 pipe, stdout/stderr flood, 정상/nonzero/denied 사례를 포함한다. 반환은 timeout+문서화한 작은 고정 종료 유예 이내이고 자식의 후속 marker가 생성되지 않아야 한다. 종료된 프로세스는 Wait/reap으로 정리하며 fixture cleanup도 둔다.
2. policy adapter에 platform별 process-group termination을 적용한다. deadline에서는 그룹을 종료하고 pipe 대기도 유한하게 만든다. 지원 플랫폼 범위는 현재 build tags와 CI를 대조해 정하고 비지원 플랫폼에 거짓 보장을 하지 않는다. 정상 작업 완료와 timeout의 exit code/TimedOut 의미를 보존한다. 대기 goroutine과 살아 있는 자식을 남기지 않는다.
3. stdout/stderr writer에 명시적 고정 보관 상한을 적용한다. writer는 넘친 bytes를 정상 drain/discard하되 원래 Write 길이를 반환하여 io.Copy가 멈추지 않게 한다. 출력 크기에 비례한 버퍼와 문자열 복사를 제거한다. redaction이 검사하지 못한 경계 tail은 보수적으로 버리고 UTF-8 safe prefix를 사용한다. 예외적으로 한 줄 전체를 버려야 하더라도 수집 상한에서 secret의 조각을 노출하지 않는다. 응답의 기존 32KiB와 truncation 표시 및 timeout 메시지 계약을 유지하고, 필요한 truncation 전달은 내부 Execution에만 둔다.
4. 경계에 걸친 가짜 token/password/URL credential 등 기존 redaction 패턴과 UTF-8, 경계 아래/정확히 상한/초과 출력, stdout/stderr 동시 flood를 검증한다. 8MiB 및 더 큰 스트림의 보관량과 TotalAlloc을 같은 fixture로 비교해 보관량과 allocation이 전체 출력량에 비례하지 않음을 증명한다. Go runtime 환경 오차 때문에 취약한 exact allocation 수치를 assertion하지 않는다.
5. scoped 정리, 해당 동작을 소유한 project doc 반영, gates 기록, 독립 diff review를 수행한다. 같은 full 검사를 중복 실행하지 않으며 Go 작업에는 process-local `GOFLAGS=-p=2 GOMAXPROCS=4`를 사용한다. 필요한 기본 battery/CI 실패는 실제 원인을 해결한다. atomic commit/push, governed draft PR, 최신 CI 성공, complete/release까지 수행한다.

## 성능 영향

정책 실행은 gates/worker/verify-work에서 공유한다. 수집 보관량은 출력 O(N)에서 채널별 고정 O(K)로 줄고 drain IO는 O(N)이다. redaction/문자열 복사는 최대 K만 처리한다. 프로세스 종료 경로에만 deadline/grace 관측을 추가한다. 8MiB와 더 큰 출력의 wall time, retained bytes, TotalAlloc을 전후 측정하며 정상 명령 지연도 확인한다. 전역 process scan이나 일반 캐시를 추가하지 않는다.

## 하위 호환성과 side effect

CLI/MCP DTO·JSON 필드, policy allow/deny, cwd/env 정책, audit metadata, timeout exit124와 일반 nonzero contract를 유지한다. durable schema와 provider body 변경은 없다. timeout 뒤 자식도 종료되는 것은 의도한 수정이다. 출력이 잘릴 때 노출되지 않는 일부 tail은 secret 경계 보호를 위한 의도한 제한이며 문서에 명시한다. rollback은 이 PR revert다. 새 플래그, legacy 경로, runner 통합 프레임워크는 만들지 않는다. worker/gates/verify-work 호출부의 응답을 회귀 확인한다.

## 게이트와 완료 기준

- G1: 프로세스와 출력 경계 회귀 | CHECK: go test ./internal/adapter/policy ./internal/application/policy ./internal/domain/policy -count=1 | EXPECT: ok
- G2: 정책 실행 경쟁 검사 | CHECK: go test -race ./internal/adapter/policy ./internal/application/policy -count=1 | EXPECT: ok
- G3: 계층 경계 | CHECK: go test ./internal/architecture -count=1 | EXPECT: ok
- G4: 응답 golden | CHECK: go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run 'Golden|TestResponseContractsGolden' -count=1 | EXPECT: ok

전체 Go/Python/self-verify 등 저장소 기본 battery는 한 번의 성공 run으로 남기고 gates와 중복 등록하지 않는다. reviewer의 필수 검증은 원장에 추가한다. 후속 marker 부재, 보관 상한, allocation 비교, UTF-8/redaction, 정상·비정상·거부·timeout에 대한 실제 실행 evidence와 성공한 CI URL/HEAD가 있어야 종료한다. native owner는 PR URL, commit SHA, CI run, lifecycle complete/release 결과를 `/tmp/issueops-five-20261001/532/owner-result.json`에 저장하고 root가 확인할 수 있게 보고한다.
