# 세션 경계와 인수 경로

`SKILL.md`의 `## 세션 경계`가 가리키는 세부 규칙이다. 본문은 단계별 실행 세션을 소유하고, 이 문서는
새 세션이 lease를 인수하는 경로를 소유한다.

새 세션은 release 확인 후 인수한다. 인수는 `replace --preview`에서 시작해 각 결과의 `next_command`를
`claim --claim-current-token`까지 따라가는 것이 유일한 경로다(해제된 lease는 `--reseed`, 죽은 active 홀더는
`--revoke` → `--finalize-preview` → `--finalize`). host 간 재개·인수도 이 generation·quiescence 검증 체인을 따른다.

## 자동 분기의 승인 범위

자동 분기는 실행 위치만 정한다. 원래 요청의 승인 범위·종료점을 그대로 유지하며,
구현만 요청한 작업에 commit·push·PR/MR 발행 권한을 추가하지 않는다.
phase·claim·`--approved`도 승인 근거가 아니다.

현재 세션 진행·새 세션·보류를 명시한 최신 사용자 지시는 자동 분기보다 우선한다.
보류는 lease를 해제하고 준비 상태를 보존한다. 기존 worktree는
세션 인계를 이유로 바꾸거나 다시 만들지 않는다.

단계 스킬과 함께 쓰는 계획·검증·Git 스킬도 이 승인 범위와 종료점을 따른다.
하위 스킬의 일반적인 "계속할까요" 절차를 추가 확인 지점으로 만들지 않는다.
