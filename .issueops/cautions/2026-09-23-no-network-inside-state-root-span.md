---
name: 2026-09-23-no-network-inside-state-root-span
description: Caution record for a solved false case or recurring risk.
---

# 전역 span 안에서 네트워크 호출을 하지 않는다

- Date: 2026-09-23
- Kind: `caution`
- Source: architecture review 2026-09-23
- Summary: phase --to pr의 git fetch와 branch retarget의 provider readback·git ls-remote가 state root 전역 span 안에서 실행돼, 원격이 느리면 같은 root의 모든 사이클 쓰기가 최대 60초 기다렸다.
- Context: sqlstore span은 cycle 단위가 아니라 state root 단위다(ADR 2026-07-07). evidence recorder도 span 안에서 git diff·status로 fingerprint를 계산해, git이 1초씩 늦어지자 무관한 사이클의 빈 span이 약 2초를 기다렸다. pr 진입은 guard와 core가 각각 fetch해 fetch가 두 번 일어났다.
- Resolution: 관측은 span 밖에서 끝내고 span 안에서는 record를 다시 읽어 관측 입력(git root, prepared base, remote artifact, requested branch)이 그대로일 때만 쓴다. pr 진입 fetch는 core가 span 밖에서 한 번 실행하고 gatesgate·loopgate guard는 자기 gate만 본다. retarget의 ls-remote는 사용자의 SSH 설정을 지키도록 주입된 GitCmd를 그대로 쓰고, 멈춰도 그 명령만 기다리게 span 밖에서 실행한다. internal/adapter/issueops/span_network_guard_test.go가 가짜 git에서 span lock을 잡아 보며 이를 고정한다. CAS 전제를 다시 확인하는 로컬 git 읽기는 span 안에 남아도 된다.
- Evidence:
  - TestPRPhaseEntryFetchesUpstreamOutsideTheSpan
  - TestBranchRetargetObservesRemoteOutsideTheSpan
  - TestBranchRetargetRejectsAnArtifactThatChangedAfterObservation
  - TestEvidenceRecordersObserveTheChangeSetOutsideTheSpan
  - internal/adapter/issueops/gatesgate/gates_gate_fetch_test.go: HEAD는 fetch 2회, 수정 후 1회
