---
name: 2026-10-23-rename-database-design-to-database-schema-architecture
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Rename database-design to database-schema-architecture and cover keys, constraints, partitioning, and ORM mapping

- Date: 2026-10-23
- Kind: `adr`
- Source: 사용자 요청(스키마 설계 의미가 이름에 드러나고, 실측 리소스·row 수·인덱스 최적화·파티셔닝·커버링 인덱스·PK·ORM 특성·외래키를 다룰 것. 이름은 `database-schema-architecture`, 저장소 전체를 새 이름으로 정리)
- Summary: 공용 스킬 `database-design`을 `database-schema-architecture`로 바꾸고, 본문과 새 `references/schema-elements.md`가 PK·FK·커버링 인덱스·파티셔닝·ORM 매핑·row 수와 테이블/인덱스 크기 실측을 다룬다. 별칭은 두지 않으며, 과거 계획·연구·이슈 문서와 품질 스냅샷의 스킬 이름도 새 이름으로 바꾼다.
- Context: 스킬 이름은 `codd`에서 `database-design`으로 한 번 바뀌었다. 본문은 관계형 스키마 설계와 쿼리 최적화를 함께 다뤘지만 PK 타입과 삽입 순서, 외래키 인덱스와 동작, 커버링 인덱스, 파티션 키와 유일성, ORM이 만드는 SQL·DDL은 다루지 않았다. 다섯 호스트의 `database-design` 링크는 모두 이 저장소의 같은 디렉터리를 가리켜 이름 충돌은 없었다. 이 저장소에서 "architecture"는 계획 단계의 `plan.architecture`, `issueops architecture review`, 공용 스킬 `architecture-review`도 가리킨다(2026-10-19).
- Decision: 1) 디렉터리, frontmatter `name`, pioneer catalog, 벤치마크·라우팅 점수 키, Go 테스트 식별자, CLI 도움말 예시, fixture(`pioneer-database-schema-architecture.json`), holdout 디렉터리와 evidence record 이름을 `database-schema-architecture`로 바꾸고, 바뀐 evidence record digest로 `evaluation-manifest.json`의 `evidence_sha256`을 고친다. 2) 사용자 지시에 따라 `.issueops/plans`·`research`·`issues`·`documentation`의 옛 이름도 바꾼다. 이 기록과 색인 행만 옛 이름을 남긴다. 3) description과 본문에 "IssueOps 계획 아키텍처 검토가 아니며 그것은 `architecture-review`"라고 적어, 이름의 architecture가 계획 단계 검토로 잘못 라우팅되지 않게 한다. 4) SKILL.md는 SURVEY·KEYS·NORMALIZE·SCALE·INDEX·ORM·OPTIMIZE·CONCURRENCY·VERIFY 순서로 방법을 정하고, Evidence Block에 `Key and constraint design` 줄을 더한다. 5) 엔진마다 다른 규칙(PostgreSQL은 외래키 참조 컬럼 인덱스를 만들지 않음, InnoDB는 만들고 secondary index가 PK를 포함, PostgreSQL 파티션 테이블의 unique·PK는 파티션 키 컬럼을 포함, Hibernate는 `IDENTITY`에서 insert batching을 끔)은 공식 문서로 확인한 것만 넣고 엔진 이름을 붙인다.
- Consequences: 사용자는 다음 `issueops update` 뒤 `/database-schema-architecture`로 호출하고, 그 전까지 호스트의 옛 `database-design` 링크는 대상이 없다. `routing-score --expect`와 `routing record --skill`은 새 이름을 쓴다. 과거 문서가 당시에 쓰지 않던 이름을 보여 주므로, 당시 이름이 필요하면 git 이력을 본다. holdout evidence는 이름을 바꾸기 전 스킬 본문으로 실행한 기록이므로, 본문 변경 뒤의 품질 주장은 새 holdout 실행이 있어야 한다.
- Evidence:
  - skills/database-schema-architecture/SKILL.md, skills/database-schema-architecture/references/schema-elements.md
  - internal/domain/pioneerskill/catalog.go, internal/domain/issueopsbenchmark/issueops_pioneer_checks.go
  - testdata/pioneer-holdouts/evaluation-manifest.json evidence_sha256
  - PostgreSQL ddl-constraints, ddl-partitioning, indexes-index-only-scans; MySQL 8.4 create-table-foreign-keys, innodb-index-types; Hibernate ORM 6.6 user guide (2026-10-23 확인)
- Alternatives / rejected options:
  - `database-design` 유지: 이름이 스키마 설계를 드러내지 못하고 범위 확장도 이름에 반영되지 않는다
  - `database-schema-design`: 한때 적용했으나 사용자가 `database-schema-architecture`를 골랐다
  - `schema-design`: API·JSON schema 설계와 구별되지 않는다
  - 옛 이름 별칭 유지: 2026-10-02(`sync-base`)와 2026-10-19(`architecture-review`) rename처럼 별칭 없이 `issueops update`가 링크를 정리한다
