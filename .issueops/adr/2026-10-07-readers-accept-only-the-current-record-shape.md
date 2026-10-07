---
name: 2026-10-07-readers-accept-only-the-current-record-shape
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Readers accept only the current record shape

- Date: 2026-10-07
- Kind: `adr`
- Source: cli
- Summary: 읽기 경로가 옛 레코드 모양을 조용히 보정하거나 허용하던 숨은 호환 동작을 지운다. 현재 writer가 항상 쓰는 필드가 없으면 읽을 때 거부한다(fail-closed).
- Context: legacy 단어와 re-export shim을 지운 뒤에도 테스트 이름(…NormalizesLegacyFailureCause 등)이 production에 숨은 호환 동작이 있음을 드러냈다. 읽기 경로가 self-verify snapshot의 failure_cause를 다시 계산했고, Orca binding의 identity version 0·빈 run_id·lease_generation 0, generation 없는 completion, purpose 없는 intent, schema_version 0 loop, selection receipt 없는 execution(2026-08-03 이전)을 받아들였다. ExecutionResult.OrcaTaskSettled/OrcaTaskError와 verify-work의 evidence 문자열은 옛 소비자를 위한 중복 출력이었다. 현재 writer는 모든 경로에서 이 필드를 기록하며, 로컬 레코드 23개는 새 reader로도 모두 읽혔다.
- Decision: 1) 위 필드는 모두 필수로 하고, 없으면 읽을 때 거부한다. 복구 체인을 두지 않는다. 2) failure_cause는 쓰기에서 분류값을 기록하고, 읽기는 검증만 한다. 3) Run 도입 전 binding을 위해 모든 Run의 task를 뒤지던 InspectOwner fallback을 지운다. 4) ExecutionResult.OrcaTaskSettled/OrcaTaskError와 verify-work evidence 문자열을 지운다. 텍스트 출력은 evidence_matrix를 쓴다. 5) project-docs 렌더러, 설치 출력, owner prompt 템플릿이 안내하던 없는 명령(hook user-prompt, worktree prepare, handoff)을 실제 hook(session-start, post-compact)으로 바꾼다. 6) 남는 것: MCP initialize revision 2025-11-25(Omo가 지금 이 revision으로 연결; revisionInitialize로 이름 변경), 설치기가 사용자 host 설정에서 우리 옛 hook 항목을 지우는 정리(2026-08-27 이전 설치본 대응), retired 이름 거부 가드.
- Consequences: 위 필드가 없는 옛 레코드는 abandon 후 다시 prepare해야 한다. 옛 self-verify-latest snapshot은 없는 상태로 취급되므로 self-verify --save-state를 다시 실행한다. verify-work --json 출력에서 evidence 배열이 빠지고 compatibility contract hash가 d500853d…로 바뀐다. MCP catalog SHA는 바뀌지 않는다.
