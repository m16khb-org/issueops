# CLAUDE.md

Claude Code에서 이 저장소를 열면 먼저 `AGENTS.md`를 읽고 동일한 규칙을 따른다.

- 공용 하네스 결정과 작업 계약: `AGENTS.md`
- 상세 문서: `.issueops/`
- Claude Code native skills: 기본은 `~/.claude/skills/*` (`atomic-commit-push`, `self-verify`, `self-augment`, `project-bootstrap`). repo-local `.claude/skills/*`는 생성하지 않는다.
- Claude Code MCP: 기본은 user-scope `issueops` 서버가 중앙 `bin/issueops mcp`를 실행하고, 그 프로세스 안에서 요청을 처리한다. 이 레포의 `.mcp.json`은 `{"mcpServers": {}}`로 비어 있다(project stdio 등록 제거). `issueops_project` stdio 항목의 템플릿은 `configs/claude/mcp.project.json`이고, `--project-local --mcp-transport=stdio` 설치 때만 `.mcp.json`에 쓰인다(HTTP 기본값에서는 오히려 제거). user-scope `issueops`와 `issueops_project`를 함께 켜면 같은 도구가 두 번 노출되므로, worktree의 새 build를 dogfood할 때만 직접 등록한 뒤 `.claude/settings.local.json`의 `enabledMcpjsonServers`로 켠다.
- 철학: 하네스 설치·업데이트·검증 경로는 독립 실행 가능해야 한다. 외부 도구가 필요하면 해당 도구의 공식 경로로 별도 설치하고, issueops는 그 설치를 대행하거나 readiness gate로 요구하지 않는다.
- 사용법은 `.issueops/OPERATIONS.md`를 따른다.

## API docs

- Endpoint/DTO/OpenAPI 변경 시 `.issueops/OPEN_API_SPEC.md`를 프롬프트로 포함하고, user-scope MCP 서버 `issueops`의 `api_doc_static_check` 후 `api_doc_review` 또는 `issueops api-doc check --json`을 사용한다.
- 대상 repo에 `npm run swagger:check`가 있으면 그 wrapper를 우선 실행한다.
