# IssueOps 실행과 정리 경계

> Family index: [ARCHITECTURE.md](../ARCHITECTURE.md).
> 사이클·actor·generation 계약은 [issueops.md](issueops.md)가 소유한다.

## Execution boundary

Workspace provisioning and lease grant are one execution transaction. The
source main worktree remains available before, during, and after direct or Orca
execution for unrelated work. A generic session binding is routing metadata
only. The fence selects the exact lifecycle ID, generation, native process
receipt, canonical worktree, and persisted Orca identity.

One active execution exists per record, not per source repository. Exact-ID
routing therefore keeps parallel cycles independent. The active holder performs
the remaining gates, implementation, publication, and completion in its
canonical worktree. Completion records `done` and releases the generation;
later merge and cleanup require separate current evidence and authority.

The merge itself is a separate, record-read-only step: `remote merge-pr` admits
only a `done` record with a released lease and a completion, reads the PR/MR once
(draft, head, checks, mergeability), refuses every blocker before mutation, and
merges pinned to the completion `final_head` (`gh pr merge --match-head-commit`,
GitLab `PUT .../merge` with `sha`). It never passes branch deletion, admin bypass,
or auto-merge, so the remote branch still flows through `cleanup remote-branch`
and an unverified merge is reported instead of assumed. It writes no record field;
cleanup re-reads the merged state from the provider.

Post-merge cleanup ordering is a contract: `reflect-completion`(사람이 쓴
진행 결과를 completion 구간에 반영. 해시·plan 원문은 record와 `.issueops/issues/<n>/`에
남는다) → `close-issue` →
`cleanup finish`. finish는 preview 게이트(원격 readback fail-closed·요청자
보호·head OID CAS·fingerprint) 뒤에만 파괴 단계를 수행하고 마지막에
레코드를 삭제한다. 워크트리를 점유한 프로세스와 그 워크트리에 매인 Orca 터미널은
차단 사유가 아니라 apply ①′의 종료 대상이다: preview가 receipt(pid·시작 시각·실행
파일)와 터미널 handle을 fingerprint에 결속하고, apply가 fingerprint된 handle마다
`orca terminal close --terminal`(same-handle·`ptyKilled=true` receipt 필수)를
호출한 뒤 HUP+TERM → KILL → 최종 점유·터미널 재관측(둘 다 0 증명) 순서로 닫은
뒤 orca 회수로 넘어간다. bulk `terminal stop --worktree`는 fingerprint 밖 동시
생성 터미널까지 닫을 수 있어 쓰지 않는다. 터미널 close 실패는 fail-closed다
(`workspace_processes_stop`, 시그널 없음). 요청자 자신(pid 조상, `ORCA_PANE_KEY`/`ORCA_TERMINAL_HANDLE`로 확정한
요청자 터미널)과 소스 체크아웃은 종료·삭제 대상이 될 수 없어 preview가 거부한다
(#477) — 결정적 ID(`sha256(repo+branch)`) 재사용과 충돌하지 않는
유일한 수명 종료다. 각 파괴 단계는 멱등이며, 실패 시 레코드가 보존되고 재실행
전 preview 재발급이 요구된다. prune은 completion 미반영 + RemoteArtifact 보유
레코드를 나이와 무관하게 보존한다(보존 불변식). staged artifact의 수명은
레코드와 같다(deleteIssueOps가 스테이지 버킷을 동반 삭제).

Cleanup `finish`, `remote-branch`, `abandon`은 같은 cycle의 실행 잠금과 `cleanup_attempt`를
공유한다. attempt의 operation·token과 관측 당시의 원본 레코드를 CAS로 결속하며,
일반 writer는 attempt가 있는 레코드를 변경할 수 없다. 각 외부 효과 직전에
소유권을 확인하고, provider 감사 반영 뒤에도 같은 레코드에만 receipt를 기록한다.
상속된 자식 프로세스가 모두 종료됐음을 drain으로 확인한 뒤 finish는 레코드를
삭제하고 remote-branch는 attempt만 해제한다. abandon은 소유한 intent 행·staged artifact·
레코드를 한 트랜잭션으로 삭제한다. 다른 operation의 attempt는 인계받지
않으며, 해당 정리 명령으로 복구해야 한다. 원격 ref가 이미 없으면 preview는
레코드를 쓰지 않는다. 같은 operation의 중단된 attempt가 남아 있을 때는 명시적인
apply가 새 token으로 인계받아 drain·해제하며, 삭제나 감사 반영을 했다고 기록하지
않는다. [공용 cleanup 소유권 결정](../adr/2026-09-29-cleanup-ownership-binds-the-operation-and-exact-record-revis.md)이
이 경계의 정규 근거다.

`abandon`의 대상 선택·사유 검증·폐기 허용 조건·자식 미완료 판정·실패 후 재시도
규칙은 `internal/domain/issueops`가 소유한다. 파일·Git·프로세스 관측은 외부에서
전달하며, 관측 실패를 자원 부재로 바꾸지 않는다. 승인 inventory는 contract DTO로,
fingerprint와 실패 기록의 봉인은 `internal/application/issueopscleanup`에서 만든다.
`close_pr`, `close_issue`, `remote_branch_delete` 실패는 로컬 삭제 전이므로,
봉인된 로컬 자원의 존재 여부와 OID가 그대로일 때만 새 preview로 재시도할 수 있다.

`AbandonExecutor`는 provider 선택과 artifact 조회 전에 공용 실행 잠금을 얻고,
그 뒤 읽은 원본 레코드에 관측 결과와 attempt를 CAS로 결속한다. CLI가 전달한
`ArtifactUnmerged` 값은 승인 근거로 쓰지 않는다. 원격 효과 → 점유 프로세스 종료 →
워크트리 제거 → 브랜치 CAS 삭제 → drain → 레코드 삭제 순서를 application이 소유하며,
각 외부 효과 전에 같은 레코드의 소유권을 확인한다. 봉인된 과거 `applying` 실패 기록은
복구 판단의 근거일 뿐, 실행 중인 다른 abandon을 통과시키는 권한이 아니다.
취소된 로컬 Git 명령은 실제 삭제를 끝냈을 수 있으므로 `applying` 실패 기록을 보존하고,
새 preview가 남은 자원을 다시 관측하게 한다. 취소된 조회의 exit code를 부재로 해석하지 않는다.

원격 폐기에서는 domain이 요청별 조회 전제 조건과 preview 효과 목록을 결정하고,
`AbandonRemoteObserver`가 허용된 provider·Git 조회를 조율한다. 어댑터는 기존
`LinkedBranchRemoteRef`를 재사용해 `ls-remote`가 반환한 ref가 요청한 브랜치와 정확히
일치하는 단일 행인지 확인한다. 성공한 빈 결과만 부재이며, 다른 ref·여러 행·불완전한
행은 `remote_branch_readable`로 거부해 fingerprint와 삭제 권한을 발급하지 않는다.

`AbandonPreviewer`가 로컬 자원·자식 cycle·pending intent·Orca owner 관측을 조합한다.
워크트리·브랜치·DB 읽기와 경로 해석은 adapter에 두고, 자식의 완료 여부와 Orca 잔여물
허용 조건은 domain에서 판정한다. preparation domain은 봉인된 단계까지 확인할 자원
순서를 결정하고, application은 그 결과를 lifecycle domain의 신원·부재 판정과 조합한다.
앞선 단계의 자원이 남았거나 부재 관측에 권위가 없으면 폐기를 거부한다. 실제 터미널은
점유 종료 단계에서 도달 가능한 경우에만 허용하며, 런타임 전환 관측 권한은 holderless일 때만 연다.

`cleanup orphan`의 요청 검증·삭제 허용 조건은 `internal/domain/issueopsorphancleanup`이,
preview·fingerprint·삭제 순서는 `OrphanCleaner`가 소유한다. 원격 병합과 Orca 관측은
잠금 밖에서 끝내고, apply는 sqlstore 쓰기 배제 잠금 아래에서 레코드·lease index와
로컬 Git 상태를 다시 읽는다. 새 소유자·잘못된 행·HEAD 변경이 있으면 삭제하지 않는다.
대상이 상태 저장소와 잠금 파일을 포함하면 symlink 경로도 정규화해 preview와 apply에서
거부한다. Git 자식은 배제 잠금을 상속하며, 취소 후 다음 삭제를 시작하지 않는다.
워크트리 제거 후 브랜치 삭제가 실패하면 부분 완료 필드를 보존한다. 성공은 drain 뒤에만
반환하며 임시 lifecycle이나 attempt를 만들지 않는다. CLI/MCP DTO와 정상 preview의
fingerprint 형식은 유지하고, 기존 orphan adapter 실행 함수·DTO 별칭·전역 Git 실행기는 제거했다.
