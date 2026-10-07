---
name: 2026-10-07-issueops-docs-layout-matches-the-append-and-runtime-paths
description: Accepted decision record with rationale, alternatives, and consequences.
---

# .issueops 폴더 구조를 append와 runtime이 쓰는 경로에 맞춘다

- Date: 2026-10-07
- Kind: `adr`
- Source: cli
- Summary: `.issueops`의 기록·폴더 배치를 현재 코드가 쓰고 읽는 경로와 같게 만들고, project-docs-optimize checker가 그 배치를 강제한다.
- Context: `project_docs_append`는 기록을 `<module_dir>/YYYY-MM-DD-<slug>.md`에 쓰는데, 예전 기록 88개는 `adr/decisions/`와 `cautions/lessons/`에 남아 같은 종류의 기록이 두 곳에 나뉘어 있었다. `.issueops/issueops/`의 이슈별 보고서 42개는 어떤 코드도 쓰거나 읽지 않았다. `operations/` 최상위에는 family module(`operations/guides`) 밖의 가이드와 평가 기록이 섞여 있었다. 이전 checker는 줄 수와 링크만 보아서 이 차이를 잡지 못했고, git이 무시하는 `evidence/`까지 검사했다.
- Decision: 1) `adr/decisions/*`를 `adr/`로, `cautions/lessons/*`를 `cautions/`로 올리고 모든 링크와 경로 표기를 고친다. 2) `issueops/<n>-*.md`와 `research/issue-19-*`·`issue-20-*` 보고서를 `issues/<n>/`로 옮긴다. 이름은 `verified-execution-report.md`, `pr-body.md`, `plan-archive.md`로 하고, runtime이 다시 쓰는 `plan.md`·`intent.md`·`spec.md`·`plan-review.md`와 겹치지 않게 한다. `issues/_unnumbered/`의 계획 25개는 `plans/`로 옮긴다. 3) 코드가 정확한 경로를 읽는 `operations/` 문서 5개(install, verification, release-reproducibility, release-dogfood-notes, quality-dashboard)는 그대로 두고, 나머지 운영 가이드는 `operations/guides/`로, skill-quality 평가와 dogfood 기록은 `research/`로, 제거된 hook 대응표는 `archive/`로 옮긴다. 4) 중첩 module(`operations/guides`) 옆에 남는 코드 고정 경로 문서는 manifest `fixed_documents`에 적는다. family module이나 runtime 폴더(`issues`, `gates`, `verified-execution`, `state`, `evidence`, `tmp`)가 아닌 최상위 폴더는 manifest `directories`에 용도를 선언하고, 표준 문서가 아닌 루트 문서는 `root_documents`에 선언한다. 5) checker는 `nested_record`, `undeclared_directory`, `undeclared_document`, `undeclared_root_document`, `missing_declared_path`, `docs_tree_ignored`를 위반으로 보고하고, git이 무시하는 파일은 검사하지 않는다. 6) 날짜가 붙은 기록과 `issues/`·`plans/`·`research/`·`archive/`의 과거 기록은 Markdown 링크 target만 고치고, 명령·증거·당시 경로 서술은 그대로 둔다.
- Consequences: 새 기록은 append 경로 한 곳에만 쌓인다. `.issueops`에 새 최상위 폴더를 만들려면 manifest에 용도를 적어야 한다. 옛 경로를 적은 외부 링크는 깨지며, 저장소 안의 링크는 같은 변경에서 모두 고쳤다. 날짜가 고정된 품질 스냅샷(`documentation/quality-*-2026-10-03.json`), `documentation/AUDIT.md`, 과거 기록 본문은 당시 경로를 그대로 보존하므로, 그 안의 경로 서술은 옮기기 전 위치를 가리킬 수 있다.
