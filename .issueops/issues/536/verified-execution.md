# #536 활성 실행 안내 검증 기록

Lifecycle: `io-d103d53f4d1a`. Branch: `536-active-contract-docs`.
구현자는 native Codex `gpt-6.1-sol/high`이며 generation 2의 canonical worktree에서 작업했다.

## 의도 대조

| 기준 | 관측 방법과 결과 |
|---|---|
| AC1: 현재 hook 권한 경계와 실행 가능한 예시 | verified-execution은 context-only hook과 CLI/MCP fence를 설명한다. G1의 `TestRetiredHookSubcommandsAreRejected`는 verbose RUN/PASS를 확인했다. |
| AC2: 감사 지침의 catalog와 단일 실행 | audit-and-process는 SessionStart project-doc catalog와 단일 deterministic pass를 요구한다. 활성 문구의 좁은 회귀 검사가 통과했다. |
| AC3: 설치 후 MCP 재연결 | install 안내를 README와 architecture/runtime에 대조했다. G2의 `TestRunMCPServesInProcessWithoutADaemon`은 verbose RUN/PASS를 확인했다. |
| AC4: self-verify 결과 설명 | loop_contract의 quick/full 및 필수 LLM gate 설명을 단일 pass, opt-in prompt, collect-all-steps의 실패 유지 설명으로 교정했다. loop metadata 회귀 검사가 통과했다. |
| AC5: 좁은 회귀 검사와 실제 named test | 세 활성 문서만 검사하는 table test와 loop result test를 추가했다. 지정 package/test는 현재 source에 존재하며 G1–G4는 모두 충족됐다. |

성공 기준과 비목표를 봉인 intent 및 계획과 대조했다. 역사 ADR·사고 기록·archive, legacy daemon/hookinput, #534 steps.go, #535 CI는 변경하지 않았다.

## 검증과 실패

- RED: `TestActiveExecutionGuidanceMatchesCurrentRuntime`와 `TestLoopContractDescribesSinglePassAndOptionalLLMEvaluation`이 기존 안내에서 의도한 이유로 실패했다.
- GREEN/CLEAN: `.issueops/issues/536/gates.md`의 G1–G4를 installed CLI의 `gates check --write`로 실행했다. validator 성공문구는 실측한 `Skill is valid!`다.
- Python 환경 실패: 기본 PATH의 Python 3.9.6은 기존 integration test의 3.10 이상 요구를 충족하지 못했다. 설치된 Python 3.14.6을 process-local PATH로 선택해 게이트 전체를 다시 실행했다.
- 중복 검증 중단: elevated risk tier가 race/vet를 소유함을 확인하기 전 별도 race/vet를 시작했다. 별도 vet는 exit 0이었으나 중복 실행을 중단했고, 중단된 self-verify/race는 완료 증거로 쓰지 않는다.
- 문서 checker: 432 documents, 6 families, violations 0. `git diff --check` 통과.
- 최종 battery의 단일 소유자는 canonical binary의 `self-verify --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json`이다. elevated tier의 full race/vet와 재사용된 go test/golden, temp build, docs/inspect를 결과 JSON에서 대조하며, 그 결과 및 PR CI는 execution completion의 verification 항목에 기록한다. 별도 전체 suite를 최종 책임으로 다시 실행하지 않는다.
- 원시 증거는 private evidence bundle의 red.log, clean-gates.json, 최종 self-verify JSON/JSONL에 보존한다. 성공하지 않은 결과를 PASS로 승격하지 않는다.

## AI slop와 측정

활성 스킬의 제거된 UserPromptSubmit/PreToolUse 권한 설명과 호환성 재시도 안내를 걷어냈다. install의 두 이벤트 공통 설치 표현도 host lifecycle context로 맞췄다. 정리 범위는 여섯 구현·검사 파일과 이슈 추적 산출물이다.

Go added nonempty lines의 shell heuristic SNR은 정리 전후 57/57 = 1.00이다. 설명 산문에는 SNR을 적용하지 않았다. 새 함수는 test 2개, production 0개이며 일반 문서 lint나 새로운 abstraction을 만들지 않았다. production 변경은 고정 문자열 다섯 줄이며 알고리즘과 hot path는 동일하다. 측정 없는 latency 개선을 주장하지 않는다.

## 문서 반영과 side effect

- audit-and-process와 install의 기존 활성 절만 수정했다. README, runtime, hook ADR, MCP ADR, Constitution, Conventions, Testing의 현재 계약을 대조했다. 새로운 설계 결정이 없어 ADR을 추가하지 않았다.
- project_docs MCP가 이 실행 환경에 노출되지 않아 project-docs-update의 CLI/direct-edit fallback을 사용했다. 코드와 ADR 원문을 근거로 해당 두 문서의 기존 안내만 교정했다.
- CLI flag, JSON key, MCP schema, state schema, 실행 동작은 유지된다. 사람이 읽는 loop_contract 배열 설명만 달라진다.
- 파일 효과는 활성 지침·회귀 검사 및 이슈 추적 자료다. durable 효과는 자기 lifecycle 기록이다. 승인된 원격 효과는 자기 branch push 및 Draft PR publication이다.
- native host 재시작, 사용자 홈·전역 설정 변경, global install은 실행하지 않았다. 임시 검증 프로세스는 종료 출력을 회수하고 최종 자식 프로세스를 확인한다.
- main 병합, issue/worktree/branch cleanup, 전역 io-update는 상위 감독 세션이 맡는다. 이 owner는 Draft PR 최신 CI 성공 및 execution complete/released에서 끝난다.
- rollback은 이 이슈 커밋 revert이며, 과거 안내가 다시 노출됨을 고려한다.

Success criteria: AC1–AC5 → G1–G4 및 최종 CLI JSON.
Evidence artifact: tracked gates 및 private run bundle, durable review/completion.
Cleanup receipt: owned interrupted verification groups의 terminal exit 회수; global runtime/resource 정리는 범위 밖.
Verification mode: focused RED/GREEN 후 단일 deterministic final battery.
Skipped checks: 브라우저·DB 검증은 해당 표면 변경이 없어 적용 대상이 아니다. global install/update는 감독 세션 소유다.
