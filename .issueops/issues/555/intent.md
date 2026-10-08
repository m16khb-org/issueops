# 요청자 의도 계약

- lifecycle: io-24725ec94e57
- issue: https://github.com/m16khb-org/issueops/issues/555
- intent_class: standard

## 원문 요청
현재 세션 스타트훅에 (SessionStart:clear says: 📚 issueops · 이 레포 project docs (관련된 것을 읽고 작업하세요) • ADR.md — ... • CONSTITUTION.md — Standalone instruction priority, safety, and accuracy principles for <다른 레포>. ...) 이런식으로 힌트가들어가고 있는데 품질 최적화 및 개선, 추가 훅 지정 등등 개선사항등에 대해서 조사해줘 claude code와 codex에 훅을 다르게 관리해야할지도 / 조사된 내용을 바탕으로 issue화를 해주고 /issueops 로 진행 / 다른 훅에는 추가될만한게 없을지도 더 조사해봐 / 하나의 이슈로 묶어서 작업하고 compact hook이나 tool use 훅에도 처리해야할게 있는지도 확인해줘

## 해석
SessionStart project-doc 카탈로그의 품질을 개선하고(모델 텍스트에 .issueops/ 경로 포함, Claude·Codex 모델 텍스트 통일, 화면 헤더에 레포 이름 표시, Claude systemMessage를 한 줄 요약으로 축소, meta.go 표준 설명을 '언제 읽는지' 형태로 개정), SubagentStart 훅을 Claude·Codex 기본 설치에 추가해 새 컨텍스트로 시작하는 서브에이전트에도 같은 카탈로그를 준다(Claude는 Explore 제외 matcher). ADR 2026-08-27을 대체하는 ADR을 남긴다. Compact·tool use 훅은 조사 결과 추가 처리 없음으로 기록한다.

## 성공 기준
- issueops hook session-start 출력의 additionalContext가 --host claude와 --host codex에서 동일하고 .issueops/ 경로를 포함한다
- Claude systemMessage는 레포 이름과 문서 개수를 담은 한 줄이고, 사용자용 헤더가 '이 레포' 대신 실제 레포 이름을 표시한다
- meta.go 표준 설명이 각 문서를 언제 읽어야 하는지 드러내며 bootstrap 렌더와 golden이 일관된다
- Claude·Codex 설치기가 SubagentStart 훅을 등록하고 업그레이드 시 co-resident 그룹을 보존하며, 훅이 hookEventName SubagentStart로 카탈로그를 낸다
- ADR이 SubagentStart 추가와 compact·tool use 훅 제외 근거를 기록하고 go test ./... 와 self-verify가 통과한다

## 비목표
- agent-harness 이름 시절 훅·MCP·skill 잔재를 설치기가 정리하는 기능 (rename 결정: 호환 alias 없음, 로컬 설정에서 직접 정리)
- PreToolUse·PostToolUse·Stop·PreCompact·UserPromptSubmit 훅 추가 (측정상 막을 위반 없음, ADR 2026-08-10/08-27 거절 유지)
- codegraph UserPromptSubmit 훅 비용 문제 (issueops 범위 밖)
- 레포별 .issueops 문서 frontmatter 수동 수정 (bootstrap --sync는 별도)

## 제약
- (없음)

## 모호함
- resolved: SubagentStart 포함 여부 — 사용자가 한 이슈로 묶어 진행하라고 지시
- deferred: Codex SessionStart systemMessage의 TUI 표시 방식 — 미확인이므로 Codex는 additionalContext만 유지

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
