# 차단 사유별 복구 절차

[`issueops-merge`](../SKILL.md)의 3단계 미리보기가 `checks_failing`·`head_mismatch`·`merge_conflict`·`merge_blocked`를 보고했을 때 읽는다.

## 체크 실패의 두 갈래

**일시적 실패**(네트워크, runner 오류처럼 코드와 무관한 실패)라면 사용자 승인을 받고
실패한 job만 다시 돌린 뒤 미리보기를 다시 실행한다. GitHub에서는
`gh run rerun <run-id> --failed`를 쓰고, GitLab에서는 provider 화면에서 실패한 job을
다시 실행한다.

**코드 결함**이라면 사이클을 다시 연다. 같은 브랜치와 같은 PR을 그대로 쓴다.

```bash
issueops execution status --id "$ISSUEOPS_ID" --json
issueops execution replace --id "$ISSUEOPS_ID" --expected-generation "$GENERATION" --preview $ACTOR_FLAGS --json
```

`$ACTOR_FLAGS`는 `issueops execution whoami --json`의 `claim_actor_flags[0]`을 쓰고(`record_actor_flags`는 거부된다), 소스 체크아웃에서
실행한다. `replace --preview`가 돌려준 `--reseed` 명령을 그대로 실행한다. reseed는 완료된
generation을 기록에 보관하고 phase를 `implement`로 되돌리며 lease를 claimable로 만든다.
보관된 PR 연결은 그대로 남는다. 이어서 워크트리 세션에 결과의 claim 명령을 넘기고,
그 세션이 `issueops next`를 따라 구현부터 `execution complete`까지 다시 진행한다.
완료가 다시 기록되면 [`issueops-merge`](../SKILL.md)의 1단계부터 다시 시작한다.

- `--expected-generation`과 fingerprint는 기억으로 채우지 않는다. `execution status`와
  preview가 돌려준 값만 쓴다.
- base가 앞서 나가 있으면 preview가 base 동기화를 먼저 요구하며 `sync-base` 명령을 돌려준다.
  그 명령을 실행한 뒤 preview를 다시 받는다.
- reseed는 완료된 실행을 복구하는 경로다. 평소 세션 인계에 쓰지 않는다.

## 충돌과 head 불일치

`base_mismatch`면 먼저 메시지의 `issueops branch retarget`으로 provider가 바꾼 base를 기록한다. 그다음은
`merge_conflict`와 같다. 쌓은 부모가 squash merge됐으면 동기화가 부모 내용을 그대로 받아들인다.

`merge_conflict`면 reseed보다 base 동기화가 먼저다. 완료된 사이클에서도 동기화할 수 있다.

```bash
issueops execution sync-base --id "$ISSUEOPS_ID" --completion-generation "$COMPLETION_GENERATION" --preview $ACTOR_FLAGS --json
```

sync-base는 replace와 달리 canonical worktree에서 실행한다(소스 체크아웃에서는
`cwd_canonical`로 거부된다). `$ACTOR_FLAGS`는 그 worktree에서 받은 `issueops execution whoami --json`
값을 쓴다. 미리보기의 `conflict_files`가 비어 있지 않으면 apply가 병합을 연 채 멈춘다(`ok: true`,
`merge_in_progress: true`). 그 파일의 충돌 표시를 양쪽 의도를 살려 고치고 `git add`한 뒤, 응답(또는 `next`)이 준
`--finalize` 명령을 실행한다. 의도가 갈려 판단할 수 없으면 `--abort`로 되돌리고 사용자에게 묻는다.
동기화가 PR head를 옮기므로 그 뒤의 미리보기는 `head_mismatch`를 보고한다. 그때 위의
reseed 경로로 사이클을 다시 열어 검증과 완료를 새 head로 다시 받는다.
