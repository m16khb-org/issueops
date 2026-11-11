---
name: 2026-11-12-cycle-views-are-one-read-only-command-the-timeline-rides-the
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Cycle views are one read-only command; the timeline rides the retained outcome row

- Date: 2026-11-12
- Kind: `adr`
- Source: claude-code main session
- Summary: 사이클 타임라인과 증거 그래프는 읽기 전용 명령 issueops view 하나로 내고, 끝난 사이클의 타임라인은 cleanup이 이미 쓰는 CycleOutcome 보존 행에 함께 싣는다.
- Context: record의 시각은 필드마다 흩어져 있고 cleanup finish·abandon은 record를 지운다. 대시보드(보드, 타임라인, 증거 그래프)를 만들려면 한 번의 투영이 필요했고, 끝난 사이클의 이력을 남길 자리가 필요했다. ADR 2026-10-24 결정 2는 MCP 전용 목록이나 next 도구를 더하지 않는다고 정했고, ADR 2026-10-23은 record를 완료 상태로 보존하는 안을 기각했다.
- Decision: 1) 새 top-level 읽기 전용 명령 `issueops view --id ID --view timeline|graph`와 issueopsview vertical(contract, domain, application, inbound)을 둔다. MCP tool은 더하지 않는다. 2) 타임라인 조립 `BuildCycleTimeline`은 domain/issueops에 둔다. cleanup 트랜잭션과 view가 같은 함수를 쓴다. 이벤트는 enum성 값만 싣고 subject·ref·phase·verdict는 `[A-Za-z0-9._:/-]` 64바이트 이하만 받는다. 그 밖의 값은 `?`가 된다. 최근 200개만 싣고 전체 개수와 표시 개수를 짝으로 낸다. 3) `CycleOutcome`에 `timeline`을 additive로 더하고 `CycleOutcomeSchemaVersion`은 1로 둔다. bucket 이름이 버전에서 파생되므로 버전을 올리면 옛 행이 보이지 않는다. 4) 증거 그래프의 stale은 다시 계산하지 않는다. 같은 프로세스에서 next 핸들러를 한 번 부르고 그 missing의 `*_stale` 키를 edge에 대응시킨다. 상태는 stale, not_reported, unknown 셋이다. 판정 가능한 stage는 allowlist이고 그 밖의 stage는 unknown이다. 5) HTML은 core가 아니라 스킬 스크립트 `skills/issueops/scripts/render_board.py`가 그린다. JS, 외부 URL, 서버, 빌드 체인을 쓰지 않는다. 6) list는 저장하지 않는 파생값 `last_record_at`과 `idle_days`를 싣고, next는 고른 사이클이 7일 이상 기록이 없으면 경고 한 줄을 싣는다.
- Consequences: response-contract golden에 `issueops_list`, `issueops_review_metrics`, `issueops_view_timeline`, `issueops_view_graph`가 생겼고 명령 목록이 늘어 contract hash가 바뀌었다. 보존 행은 사이클당 최대 약 40KB 커진다(상한 64KiB를 테스트가 고정한다). 옛 binary는 새 보존 행을 읽을 수 있고, 옛 행은 `retained_without_timeline`으로 읽힌다. not_reported는 신선도 증명이 아니다. next가 막는 첫 게이트만 보고하기 때문이다. next의 7일 임계값은 측정 분포가 없는 판단값이므로 실제 사이클의 idle_days 분포를 모은 뒤 다시 정한다. next에 stage가 늘면 `TestGraphStagesClassifyEveryNextStage`가 분류를 요구한다.
- Evidence:
  - commits b511f53d4, cae9c0666, 13e71d77f, 56fdda64c, 620c9fe9a
  - internal/domain/issueops/cycle_timeline.go, internal/domain/issueopsview/graph.go, internal/application/issueopsview/service.go
  - go test (WSL Ubuntu 26.04, go1.26.6): gofmt, vet, lint, race, self-verify exit 0 at the merged head
  - .issueops/research/2026-11-11-artex-orchestration-visualization.md
- Alternatives / rejected options:
  - status 확장(status --view): status vertical은 record만 읽는데 graph는 next의 git 관측과 --cwd가 필요하고, 보존 타임라인을 읽으려면 status의 not-found 경로를 바꿔야 한다
  - 스킬 스크립트가 status --json을 읽어 계산: cleanup 트랜잭션이 Go에서 타임라인을 써야 하므로 같은 로직이 두 언어에 생긴다
  - 명령 두 개(timeline, evidence-graph): catalog, usage, parser, golden 비용이 두 배다
  - review domain 함수로 stale을 다시 계산: next의 관측 조합을 복제해야 하고 next가 그 단계에서 판정하지 않는 게이트까지 판정해 next와 달라진다
  - CycleOutcome schema 버전 올리기: 새 bucket이 생겨 v1 행을 읽는 곳이 모두 두 bucket을 읽어야 한다
  - cleanup finish 전에 .issueops/issues/<n>/에 파일 쓰기: 정리 순서 계약에 쓰기 단계가 생기고 머지 뒤 source checkout을 더럽힌다
  - lease 인계 이력의 append-only 필드: record schema 변경이고 수요 관측이 없다
  - core에 html/template: CLI 출력 계약은 JSON/text다
