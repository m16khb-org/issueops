---
name: 2026-10-08-claude-agents-self-verify-met
description: Caution record for a solved false case or recurring risk.
---

# claude --agents 위치 인자 흡수, 새 스킬의 로컬 self-verify, 증거 문서의 홈 경로, met 게이트 재실행

- Date: 2026-10-08
- Kind: `caution`
- Source: issueops-docs #553
- Summary: #553에서 네 가지를 만났다: claude print 모드의 --agents가 뒤따르는 프롬프트를 값으로 삼킨다, 스킬을 추가한 사이클의 로컬 self-verify는 홈 설치본과 install-native.sh의 stable root 때문에 실패한다, 추적되는 증거 문서의 절대 홈 경로가 식별 데이터 검사에 걸린다, gates check는 met 게이트를 다시 돌리지 않는다.
- Context: #553(2026-10-08) 역할 모델 설정 구현 중. (1) 실측에서 `claude -p --model haiku --agents "<json>" "<prompt>"`가 `Input must be provided either through stdin or as a prompt argument`로 끝났다(Claude Code 2.1.293). (2) 게이트 원장의 self-verify가 `native integration failed: missing ~/.claude/skills/io-model/SKILL.md`로 실패했다. 사용자 홈 설치본은 base이고, 임시 HOME에서 install-native.sh를 돌려도 연결 워크트리를 메인 체크아웃(ResolveStableNativeRoot)으로 바꿔 설치해 새 스킬이 빠졌다. (3) 임시 HOME 설치가 풀리자 Python 단계의 meeting_notes_skill_contract_test가 새 `.issueops/issues/553/live-check.md`의 `/Users/<사용자명>/...` 경로를 식별 데이터로 잡았다(2026-10-03 기록과 같은 종류). (4) 정리로 코드를 바꾼 뒤 `issueops gates check --write`를 다시 돌려도 이미 met인 게이트는 실행되지 않았다.
- Resolution: (1) print 모드에서 `--agents` 뒤 프롬프트는 `--` 다음에 두거나 표준 입력으로 넘긴다. cmux argv는 `--agents <json> -- <prompt>` 순서이고, Orca owner 명령은 위치 인자 프롬프트가 없다. (2) 스킬을 추가·삭제한 사이클의 로컬 self-verify는 사용자 홈을 고치지 않는다. worktree의 추적·미추적 파일을 임시 독립 저장소로 복사해 커밋하고, 그 안에서 go build → `HOME=<tmp> CODEX_HOME=<tmp>/.codex ./scripts/install-native.sh --skip-build --path-mode=skip --mcp-transport=stdio` → self-verify를 실행한다. GOCACHE·GOMODCACHE·GOPATH는 실제 값을 넘긴다. #553 원장 G12가 그 한 줄 CHECK다. (3) 추적되는 증거 문서에는 `<source root>`, `<canonical worktree>`, `$WORKSPACE` 같은 자리 표시를 쓰고 홈 절대 경로를 적지 않는다. (4) 코드가 바뀐 뒤에는 원장의 `- [x]`와 EVIDENCE를 pending으로 되돌린 다음 check를 다시 실행한다. EXPECT는 고치지 않는다.
- Evidence:
  - .issueops/issues/553/live-check.md
  - .issueops/issues/553/gates.md (G12)
  - internal/adapter/install/install.go (ResolveStableNativeRoot)
  - scripts/meeting_notes_skill_contract_test.py
  - internal/domain/gates/check_decision.go (ShouldRun)
