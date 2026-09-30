---
name: 2026-09-30-ddd-responsibility-ownership-across-the-harness
description: Accepted decision record with rationale, alternatives, and consequences.
---

# DDD responsibility ownership across the harness

- Date: 2026-09-30
- Kind: `adr`
- Source: DDD whole-project refactor
- Summary: 업무 판단은 domain, 실행 순서와 트랜잭션 조율은 application, 외부 효과는 adapter가 소유한다.
- Context: 프로젝트 전체 DDD 책임 분리와 legacy 중계 제거 요청에 따라 CLI·MCP·설치·작업 사이클의 production 호출 경로를 이전했다. main의 본문 가독성·추적 계획 사본·완료 보고 기능도 같은 경계로 통합했다.
- Decision: contract는 공개 DTO·codec을 유지한다. domain은 관측값을 입력받아 규칙과 상태 전이를 판단하고, application은 좁은 port로 관측·검증·외부 효과·저장 순서를 조율한다. composition root가 인스턴스별 의존성을 연결하며 CLI와 MCP는 parse/render/dispatch를 담당한다. 저장 adapter는 raw CAS·관련 행 원자성·잠금·process drainage를 보존한다. 본문 가독성 보고 DTO는 contract/artifactreadability에 두고, tracked materials의 복사는 application/issueopsremote가 담당한다. cleanup은 이슈 본문을 다시 쓰지 않는다.
- Consequences: 새 production 파일·심볼은 책임 원장에 배정하며 금지 의존성 예외를 늘리지 않는다. 최종 발행에는 전체 Go·race·lint·격리 설치 검증과 독립 리뷰를 적용한다. capability별 상세 책임은 architecture/domain-responsibilities.md와 architecture/issueops-cleanup.md를 따른다.
- Evidence:
  - internal/architecture/testdata/ddd_responsibility_inventory.json
  - internal/architecture/ownership_manifest_test.go
  - internal/application/issueopsremote/tracked_materials.go
  - internal/contract/artifactreadability/report.go
  - .issueops/evidence/ddd-refactor/T22-final-isolated-evidence.json
- Alternatives / rejected options:
  - 폴더만 이동하거나 예전 함수·전역 setter를 facade로 남기는 방안은 책임 중복과 인스턴스 간 의존성 누출 때문에 제외했다.
  - 공개 저장 schema를 전면 변경하는 방안은 필요하지 않아 제외하고 schema 1과 기존 reader 거부 규칙을 유지했다.
