---
name: 2026-10-08-parallel-tests-and-non-blocking-flock-leases
description: Caution record for a solved false case or recurring risk.
---

# Parallel tests and non-blocking flock leases

- Date: 2026-10-08
- Kind: `caution`
- Source: issueops-docs io-7dff0c24f1e9 (#558)
- Summary: processlease의 LOCK_NB flock 경로에 닿는 테스트는 t.Parallel로 돌리면 다른 테스트의 fork/exec가 잠금 fd를 잠깐 복제해 'execution lifetime is still active'로 확률적으로 실패한다.
- Context: #558에서 internal/adapter/issueops 테스트에 t.Parallel을 넣었을 때 race+shuffle 첫 실행에서 cleanup 테스트 약 20개가 실패했다. env·cwd·패키지 변수 기준의 call graph로는 이 공유 상태를 찾지 못했다. runtime.MemStats를 재는 테스트도 다른 병렬 테스트의 할당이 섞여 실패했다.
- Resolution: processlease.Acquire/Drain(CleanupLifetimeLock, sqlstore write exclusion 경로)에 닿는 테스트 135개와 MemStats 테스트를 직렬로 남겼다. fork/exec는 exec 전까지 부모의 fd를 복제하므로 O_CLOEXEC로도 그 짧은 창을 막지 못한다. 병렬화 대상을 고를 때 env·cwd·전역 변수와 함께 flock 기반 lease와 프로세스 전역 측정(MemStats, GC, NumGoroutine)도 제외 기준에 넣는다. 병렬화 뒤에는 go test -race -shuffle=on -count=3으로 확인한다.
- Evidence:
  - internal/adapter/outbound/processlease (LOCK_NB flock)
  - go test -race -shuffle=on -count=3 ./internal/adapter/issueops: 제외 전 실패, 제외 뒤 연속 통과
  - TestWorkspaceSnapshotStreamsLargeUntrackedFiles (MemStats TotalAlloc)
- Alternatives / rejected options:
  - 잠금을 blocking으로 바꿔 경합을 흡수: production lease 의미가 바뀐다
  - 실패한 테스트만 골라 제외: 실패는 확률적이라 다음 실행에서 다른 테스트가 깨진다
