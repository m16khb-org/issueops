---
name: cautions/2026-10-02-real-host-qa-tool-results-service-tier-amfi-exact-edits.md
description: Dated lesson — real-host MCP QA pitfalls: Claude tool-results spill files, Codex 0.128.0 service_tier rejection, AMFI-killed extracted binaries, and exact-replacement edits when apply_patch is stale.
---

# 2026-10-02 — 실제 host QA에서 만난 함정 네 가지

Family index: [CAUTIONS.md](../CAUTIONS.md).

- Kind: `caution`
- Source: agent improvements I1-I10 실제 host 검증(`.issueops/plans/agent-improvements-2026-10-02/host-qa-preflight.md`, `parent-verification.md`)
- Summary: 세 host로 공용 HTTP MCP와 stdio를 실측하는 동안 제품 결함이 아닌 실패가 네 번 나왔다. 넷 모두 결과를 제품 실패나 성공으로 잘못 기록하기 쉬운 경우다.
- Context:
  - Claude Code 2.1.287은 기본 출력 한도를 넘는 tool 결과를 자기 세션의 tool-results 파일에 저장하고, 대화에는 그 파일을 가리키는 안내문만 넣는다. `docs_index` 응답(288828자)에서 이 동작이 나왔고, 초기 QA parser는 안내문을 JSON으로 읽다가 실패했다.
  - Codex 0.128.0은 `service_tier`로 `fast|flex`만 허용한다. 사용자 `$HOME/.codex/config.toml`에 `service_tier = "default"`가 있으면 `codex exec`는 설정을 읽는 단계에서 exit 1로 끝나고 출력은 0바이트다. `-c service_tier="flex"` override는 파싱 뒤에 적용되므로 효과가 없다.
  - npm public registry에서 받은 Codex darwin-arm64 native 바이너리를 풀어 실행하자 `codesign --verify`는 통과했지만 실행은 SIGKILL(137)로 끝났다. AMFI가 unsigned code로 판정했기 때문이며, 새 inode로 복사해도 결과가 같았다.
  - 세션의 `apply_patch` 도구가 reload 뒤 stale 상태여서 편집을 적용하지 못했다.
- Resolution:
  - Claude: 한도를 올리거나 제품 출력을 줄이지 않는다. 현재 native 세션의 tool-results artifact 경로만 허용하고, 저장된 파일의 JSON을 파싱해 판정한다.
  - Codex service_tier: 전역 설정을 고치지 않는다. `auth.json`을 격리 `CODEX_HOME`에 복사하거나 symlink하지도 않는다. 토큰 refresh 때 실제 로그인이 깨질 수 있기 때문이다. 확보하지 못한 export는 "미확보"로 기록하고 결정적 fixture 테스트 범위만 주장한다.
  - AMFI: 원본과 전역 설치는 그대로 두고, QA용 복사본에만 ad-hoc 서명(`codesign -s -`)을 해서 실행한다. 실행 경로는 증거에 함께 적는다.
  - 편집: `apply_patch`를 쓸 수 없으면 일치 횟수를 검사하는 정확한 문자열 치환으로 적용한다. 적용한 뒤에는 diff, gofmt, 진단 결과로 확인한다.
- Evidence:
  - `.issueops/plans/agent-improvements-2026-10-02/host-qa-preflight.md` (Claude tool-results, Codex exec `service_tier`, ad-hoc 서명한 QA 사본 `codex-cli 0.128.0`)
  - `.issueops/plans/agent-improvements-2026-10-02/parent-verification.md` (AMFI SIGKILL 137, 정확 치환 편집)

> Incident-time command, field, and state references are historical evidence, not current execution directives.
