---
name: cautions/issueops-stages.md
description: Cautions for the ten-stage IssueOps boundaries, execution mode selection, worktree adoption, unmerged retirement, and fingerprint sealing order.
---

# IssueOps stage cautions

Family index: [CAUTIONS.md](../CAUTIONS.md). 10단계 흐름을 운영하면서 반복해서 밟은 함정이다. 단계 경계와 봉인 순서에서
생기는 문제만 여기 둔다. lease·fence 문제는 `issueops-execution.md`, 브랜치·게이트
문제는 `issueops-lifecycle.md`가 소유한다.

## 1. execution mode와 세션 런처를 혼동하지 않는다

증상: 새 세션을 열려고 이미 준비된 direct execution을 `auto|orca`로 바꾸거나,
Orca·Herdr의 worktree 생성 명령으로 canonical worktree를 다시 만든다.

원인: workspace/lease 운영 방식과 native 세션 배치를 같은 결정으로 취급했다.

규칙: 일반 경로는 사유를 포함한 `execution prepare --mode direct`로 준비하고,
`session-choice.md`에 따라 Orca, Herdr, 현재 세션 순서로 결정한다. 같은 worktree와
mode를 유지하며 release 뒤 새 세션 하나에 인계한다. 명시적으로 요청했거나 기존에
선택된 Orca execution의 core 경로와 `auto|orca` API 의미는 유지한다.

Herdr 0.9.0은 Claude의 첫 MCP 선택 화면을 `idle`로 보고한 사례가 있다.
`interactive_ready`나 prompt 제출 성공만 믿지 말고 실제 입력 화면과 native 수신 기록을
확인한다. 모호한 전달 결과는 같은 세션에서 조사하며 다른 host로 대체하지 않는다.

근거: `skills/issueops-plan/SKILL.md` 인계 절,
`skills/issueops/references/session-choice.md`의 환경 판별과 Herdr 실행 절.

## 2. 워크트리 세션은 `<source>.worktrees` 쓰기 권한이 필요하다

증상: Orca나 relaunch로 띄운 세션이 canonical worktree를 만들거나 고치지 못한다.

원인: 세션의 작업 디렉터리가 source checkout이고, 형제 경로인 워크트리 루트가
접근 허용 목록 밖이다.

규칙: relaunch 명령은 워크트리로 `cd`한 뒤 실행하고, 접근 허용이 필요한 호스트는
`--add-dir`로 워크트리 루트를 함께 넘긴다. 권한이 없으면 prepare 전에 멈춘다.

근거: `internal/adapter/gitworktree/provisioner.go` `workspaceRelaunchCommand`.

## 3. 봉인한 base SHA 위에 커밋하면 채택이 깨진다

증상: `execution prepare`가 `existing canonical worktree identity does not match
branch and base_head`로 실패한다.

원인: 워크트리를 미리 만들고 그 위에 커밋했다. 채택은 top-level·branch·HEAD 세
조건이 모두 맞을 때만 성립한다.

규칙: 워크트리 provisioning은 `execution prepare`가 단독으로 소유한다. 앞 단계는
브랜치 정체성만 기록한다. 이미 커밋한 워크트리는 그 커밋을 base SHA로 기록하거나
워크트리를 지우고 prepare가 만들게 한다.

근거: `internal/adapter/gitworktree/`의 채택 검사, `skills/issueops-prepare/SKILL.md` 안전 규칙.

## 4. 미머지 사이클의 원격 정리는 `cleanup abandon`만 할 수 있다

증상: `remote close-issue`가 아티팩트 검증 실패로, `cleanup remote-branch`가
`phase_done` 미충족으로 막힌다. 그 사이클은 출구가 없어 보인다.

원인: 두 명령 모두 머지 증적을 하드 게이트로 요구한다. 그리고 `cleanup abandon
--apply`는 record를 지우므로 그 뒤에는 `--id` 기반 명령이 동작하지 않는다.

규칙: 미머지 사이클은 `cleanup abandon --close-pr --close-issue
--delete-remote-branch`로 원격 효과를 opt-in한다. 원격 효과는 record 삭제보다 먼저
실행된다. 머지된 사이클은 `issueops-cleanup`의 reflect → finish가 맞다.

또한 implementation review 게이트는 2026-09-05부터 모든 execution mode에 적용된다.
진행 중이던 direct 사이클도 pr 진입과 create-pr 전에 리뷰 기록이 필요하다.

근거: `internal/adapter/issueops/issueops_cleanup_abandon_remote.go`, `skills/issueops-abandon/SKILL.md`.

## 5. strict readiness는 `git fetch`를 실행한다

증상: 단계 판별처럼 자주 부르는 읽기 전용 경로가 원격을 때리고, 원격이 느리거나
불가할 때 그 경로 전체가 느려지거나 실패한다.

원인: `IssueOpsStrictPRReadiness`는 upstream 동기화를 판정하므로 fetch가 필요하다.

규칙: 읽기 전용 판별에는 `IssueOpsLocalPRReadiness`를 쓴다. fetch와
`upstream_fetch`·`upstream_synced` 판정만 빠진 같은 관측이다. `issueops next`는 이
표면만 쓰며 네트워크를 호출하지 않는다. 커밋·푸시 직전의 strict 판정은 8단계가
명시적으로 실행한다.

근거: `internal/adapter/issueops/issueops_pr_readiness_strict.go`.

## 6. change fingerprint는 변경·untracked 파일 전체를 덮는다

증상: 정리를 봉인한 뒤 문서 한 줄만 고쳤는데 `ai_slop_clean_stale`로 pr 진입과
create-pr이 막힌다.

원인: fingerprint는 `git diff base..HEAD`와 `git status --porcelain`이 가리키는 모든
경로의 내용 해시다. gates.md, verified-execution report, 운영 문서, untracked 파일이 전부 들어간다.

규칙: 파일을 바꾸는 작업은 4·5단계에서 끝낸다. 6단계가 운영 문서를 고쳤으면 그
단계가 `ai-slop-clean record`로 재봉인한 뒤 판정을 기록한다. 7단계 검증은 파일을
만지지 않는다. 되돌아온 것은 오류가 아니라 의도된 회귀다.

근거: `internal/adapter/issueops/implementation/evidence.go`, `skills/issueops-clean/SKILL.md` 기록 절.
