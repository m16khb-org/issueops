# #536 활성 실행 안내를 현재 계약에 맞춘다

- Lifecycle: io-d103d53f4d1a. Issue: https://github.com/m16khb-org/issueops/issues/536
- Source: $HOME/workspace/issueops. Base: main b917fc960b72bca3e616df1162c6b89d40a517ba.
- Branch: 536-active-contract-docs. Canonical worktree: $HOME/workspace/issueops.worktrees/536-active-contract-docs.
- 최신 사용자 요청은 다섯 작업의 병렬 구현 인계와 main 병합, cleanup, io-update까지다. 이전 감사 전용 제약은 최신 intent와 scope 결정으로 대체했다.
- 구현자는 gpt-6.1-sol/high Codex native 세션이다. 이 owner의 종료점은 검증한 Draft PR, 최신 push 및 PR CI 성공, execution complete/released다. 병합·cleanup·전역 설치는 상위 세션이 맡는다.

## 문제와 성공 기준

현재 활성 지침에 실제 코드가 제거한 동작이 남아 있다. `skills/verified-execution/SKILL.md:58-64,93-99`의 lifecycle 테스트 예시와 hook 권한차단 설명, `.issueops/cautions/audit-and-process.md:19,40`의 10회 하한과 UserPromptSubmit catalog 금지, `.issueops/operations/install.md:50-52`의 daemon 재시작에 의한 MCP 자동 갱신, `internal/application/selfverify/loop.go:52-53`의 quick/full 및 필수 LLM 설명을 교정한다. 근거는 현재 `cmd/issueops/hookcli/hook.go:19-41`, `internal/application/update/daemon.go:33-43`, `internal/application/selfverify/loop.go:67-68`와 관련 ADR이다.

완료 조건은 이 네 표면이 현재 context-only hook, CLI/MCP 소유 fence, host MCP 재연결, 단일 deterministic pass 및 opt-in LLM 동작과 일치하고, 대체 예시 테스트가 실제 이름으로 실행되며, 좁은 회귀 검사가 다시 나타나는 모순을 잡는 것이다. quick/full 모드 도입, hook 부활, daemon 삭제, 날짜가 있는 역사 기록 정리는 범위 밖이다.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 제0장과 제5장: 현재 코드로 계약을 확인하고 문서를 맞추며 CLI 설명 변경은 실제 JSON과 검증한다.
- `.issueops/ARCHITECTURE.md` IssueOps ownership 및 `.issueops/architecture/runtime.md` 실행 모드: 권한은 CLI/MCP가 소유한다. 현재 MCP는 in-process이고 daemon은 옛 proxy용이다.
- `.issueops/CONVENTIONS.md` State / policy / guard / hook / lifecycle: SessionStart context-only, enforcement hook 없음. 스킬은 shared source 하나를 유지한다.
- `.issueops/ADR.md` accepted baseline와 `.issueops/adr/decisions/2026-08-27-session-start-owns-compaction-context.md`: SessionStart catalog 계약이 우선하며 역사 결정은 보존한다.
- `.issueops/adr/2026-09-23-issueops-mcp-serves-in-process-the-shared-daemon-leaves-the.md`: update는 daemon stop/cleanup만 하고 host 프로세스 재연결이 필요하다.
- `.issueops/CAUTIONS.md` Dated 기록 구분 및 `.issueops/cautions/audit-and-process.md` 관련 절: 과거 기록과 활성 지시를 구분한다. 폐기 모드의 날짜가 있는 역사 설명은 삭제 대상이 아니다.
- `.issueops/TESTING.md` 최소 완료 및 `testing/self-verification.md`: 단일 검증 battery와 실제 named test 실행을 증명한다. 실패 뒤에는 원인을 고친 최종 revision에서 순서대로 다시 검증한다.

## 재사용하는 기존 구현

- README의 MCP 재연결 안내와 architecture/runtime의 현재 설명을 참조한다. 설치 문서에 별도 lifecycle 규칙을 만들지 않는다.
- `TestRetiredHookSubcommandsAreRejected`(cmd/issueops/hookcli), `TestRunMCPServesInProcessWithoutADaemon`(cmd/issueops/mcpcli)을 실제 named example 및 smoke 증거로 재사용한다.
- `internal/adapter/skillcontract/skill_contract_test.go`의 repo-root/문서 읽기 helper와 `internal/application/selfverify/loop_test.go`의 result 검증을 재사용한다. 문구 전체 snapshot 대신 실제 제거 계약의 재등장과 필수 현재 계약 누락을 좁게 검사한다.
- 회귀 검사는 네 활성 파일만 다룬다. 레포 전체 banned-word grep나 일반 문서 lint 프레임워크는 만들지 않는다.

## 실행 단계

1. canonical worktree에서 실제 현재 소스와 audit 증거를 대조하고 최소 회귀 검사를 먼저 추가한다. 테스트: 활성 지침은 context-only hook/CLI fence를 설명하고, named example의 package와 test가 존재하며, 설치 문서가 host MCP reconnect를 요구하고, loop_contract가 one-pass/opt-in LLM을 설명해야 한다. 현재 문구로 실패하는 RED를 보관한다. 날짜 있는 history 및 legacy backend 유지는 수동 diff 검토로 확인한다.
2. 네 활성 파일과 필요한 좁은 테스트만 수정한다. verified-execution의 남아 있는 현재형 enforcement/compatibility repair 문단도 같은 폐기 계약이면 함께 제거·정정한다. 일반 lease, actor, 안전한 인계 지침은 보존한다. loop_contract는 문자열 설명만 바꾸며 JSON field/schema/실행 동작은 바꾸지 않는다. 종료·score·collect-all-steps 설명도 실제 구현과 대조해 모순 없는 내용으로 표현한다.
3. named focused tests를 `-v`로 실행하고 `=== RUN`과 PASS를 확인한다. 새로 추가한 문서 예시도 같은 방식으로 실행해 zero-match가 성공 증거가 되지 않게 한다. skill validator 및 project-doc checker, docs/inspect, build와 self-verify 실제 JSON에서 iterations=1 및 현재 loop_contract를 확인한다.
4. IssueOps slop-clean → docs → verify로 최종 diff를 봉인하고 독립 리뷰를 받는다. 해당 작업 기준 전체 Go/race/vet/golden 및 self-verify battery를 한 최종 revision에서 완료하고 evidence를 원장에 남긴다. 문서 사본/intent/plan-review/gates를 포함하여 Conventional Commit + Lore로 커밋·푸시하고 Draft PR을 게시한다. 최신 push/PR CI 성공과 원격 artifact verification 후 execution complete/released로 종료한다.

각 단계는 앞 단계에 의존한다. 실패 시 원인을 좁혀 수정하며 동일한 긴 suite를 근거 없이 반복하지 않는다. 병렬 sibling #534는 selfverify steps.go golden/drift 동작, #535는 CI 중복 검증을 소유한다. 이 작업은 steps.go와 CI 파일을 수정하지 않는다. 충돌 가능성이 발견되면 상위 세션에 공유한다.

## 게이트 원장 계획

- G1: 현재 hook surface | CHECK: go test -v ./cmd/issueops/hookcli -run ^TestRetiredHookSubcommandsAreRejected$ -count=1 | EXPECT: PASS
- G2: in-process MCP | CHECK: go test -v ./cmd/issueops/mcpcli -run ^TestRunMCPServesInProcessWithoutADaemon$ -count=1 | EXPECT: PASS
- G3: 활성 지침과 loop metadata 회귀 | CHECK: go test -v ./internal/adapter/skillcontract ./internal/application/selfverify -count=1 | EXPECT: PASS
- G4: skill 구조 | CHECK: python3 scripts/validate-skill.py skills/verified-execution | EXPECT: (validator의 실제 성공 출력으로 implement 단계에서 봉인)

각 EXPECT는 implement 진입 전 실제 도구 출력 계약으로 확정한다. G1/G2의 verbose 이름 실행도 evidence에서 확인한다. 최종 단일 battery는 testing/self-verification.md의 정규 목록을 한 번 등록하며 동일 검사를 별도 목록에 중복 등록하지 않는다. API DTO 변경은 없지만 loop_contract JSON 설명 변경으로 response-contract/golden 및 실제 self-verify JSON을 대조한다.

## 성능 영향

사용자 실행 hot path나 알고리즘을 바꾸지 않는다. loop result의 고정 문자열만 바뀌어 시간·공간 복잡도는 동일하다. 좁은 contract 테스트만 추가한다. 다섯 병렬 작업의 호스트 자원 경합을 줄이도록 process-local GOFLAGS=-p=2, GOMAXPROCS=4를 사용하고 전역 환경이나 설정은 바꾸지 않는다. full self-verify는 canonical source와 binary가 일치하는 최종 증거용으로 실행한다.

## 하위 호환성과 side effect

CLI flag, JSON key, MCP schema, state schema와 provider body 의미는 유지한다. loop_contract는 사람이 읽는 설명 배열만 갱신하며 동작 보장을 정확히 한다. 현재 legacy daemon 및 hookinput 코드와 역사 ADR/사고 기록은 보존한다. 런타임 hook/MCP 세션을 재시작하거나 전역 install을 하지 않는다. shared skill 지침이므로 모든 호스트에 동일하게 반영된다. 원격 부작용은 승인된 자기 issue 기록과 branch/PR publication뿐이고, 상위 세션이 merge/cleanup/update를 담당한다. 롤백은 이 커밋을 revert하되 오래된 설명이 다시 노출됨을 인지한다. 이 변경은 절차 산문 교정이며 새 LLM prompt template을 만들지 않는다.
