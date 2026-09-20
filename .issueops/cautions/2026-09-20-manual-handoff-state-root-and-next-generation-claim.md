---
name: 2026-09-20-manual-handoff-state-root-and-next-generation-claim
description: Caution record for a solved false case or recurring risk.
---

# Manual handoff state root and next-generation claim

- Date: 2026-09-20
- Kind: `caution`
- Source: actual parallel session handoff dogfood
- Summary: 수동 인계 관측은 lifecycle과 같은 state namespace를 사용하고 released 세대와 새 claim 세대를 구분해야 한다.
- Context: 2026-09-20 실제 direct 사이클 두 개를 release한 뒤 trace handoff-delivery가 존재하는 lifecycle ID를 찾지 못해 프롬프트 전달 전에 중단됐다. producer는 상위 StateDir을 읽었지만 lifecycle과 claim observer는 IssueOpsStateRoot를 사용했다. 같은 세대만 비교하는 claim correlation도 실제 reseed의 세대 증가를 반영하지 못했다.
- Resolution: 수동 producer의 record 조회와 audit 저장을 IssueOpsStateRoot로 통일했다. manual-direct 관측은 source generation N과 실제 claim N+1을 연결하고 일반 Orca 관측은 같은 세대 규칙을 유지한다. host·PID·시작 시각·실행 파일·단일 lineage 검사는 유지한다. 관측은 권한을 만들지 않으며 claim CAS가 권한의 근거다. 실제 운영 경로의 namespace와 reseed→claim을 검증해야 임의로 같은 root·generation을 넣은 테스트가 숨기는 결함을 잡을 수 있다.
- Evidence:
  - cmd/issueops/issueopsapp/handoff_delivery_wiring_test.go
  - cmd/issueops/issueopsapp/issueops_claim_wiring_test.go
  - internal/domain/issueops/handoff_delivery_test.go
  - actual trace: issueops record io-75f917ecd7a1: file does not exist; prompt not sent
  - focused regression: released direct generation 1 -> real reseed generation 2 -> real claim; strict identity mismatch and ambiguity rejection
