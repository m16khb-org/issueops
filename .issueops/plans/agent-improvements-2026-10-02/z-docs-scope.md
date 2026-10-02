# Z 문서·계약 갱신 범위 (읽기 전용 조사 결과)

공용 Streamable HTTP MCP 방향과 어긋나는 서술을 grep으로 찾았다(`in-process|stdio server|47831|Streamable`).
아래는 Z-integration이 project_docs 계약으로 고쳐야 할 위치다. 생성된 openwiki는 제외한다.

| 문서 | 위치 | 필요한 변경 |
|---|---|---|
| `.issueops/ADR.md`, `.issueops/adr/README.md:29` | 2026-09-23 in-process 결정 색인 | 새 ADR 추가: 로컬 공용 HTTP 서비스 + native 발급 caller capability, stdio는 호환 표면. 2026-09-23 결정은 stdio 경로 설명으로 범위를 좁힌다 |
| `.issueops/ARCHITECTURE.md:52` | 실행 모드 요약 | HTTP 서비스 추가, 요청별 scope/authority |
| `.issueops/OPERATIONS.md:53` | "MCP stdio server inside the host session" | `mcp --http`, `mcp service`, `mcp authorize`, host HTTP 설정, `--mcp-transport=stdio` |
| `.issueops/TECH_STACK.md:31,99` | 실행 모드, SDK v1.6.1 | SDK v1.8.0, HTTP transport |
| `.issueops/architecture/runtime.md:15`, `architecture/hexagonal-core.md:24` | stdio server 행·다이어그램 | HTTP 서비스 행과 세 host 직접 연결 |
| `.issueops/cautions/runtime.md:41`, `cautions/audit-and-process.md:39` | in-process 전제 | 서비스 build_id·supervisor 주의, 서버 cwd 비사용, credential 파일 경계 |
| `AGENTS.md` §5 표·§8 명령 목록 | 통합 표면, top-level 명령 | 공용 HTTP 1차 표면 결정과 `mcp authorize/service/--http` |
| `README.md` | 설치·사용 안내 | HTTP 기본, stdio 선택 |
| `cmd/issueops/testdata/mcp_tools.golden.json`, response golden, DDD inventory | 공유 golden | L 결정 뒤 한 번 재생성·등록 |

DDD inventory: `internal/architecture/ddd_responsibility_test.go`의
`go test ./internal/architecture -run TestDDDResponsibilityInventoryMatchesSource -update-ddd-inventory`로
`testdata/ddd_responsibility_inventory.json`을 재생성한다. owner는 경로 prefix(contract/domain/
application/port/adapter/composition/inbound), task는 `dddTask`의 경로 규칙에서 파생되므로
새 파일에 특별한 task가 필요하면 그 표에 prefix를 추가한다. policies 블록은 보존된다.
재생성 뒤 diff를 읽어 의도치 않은 심볼 삭제가 없는지 확인한다.

기록할 사실: Codex 2025-06-18, Claude 2026-07-28(server/discover, stateless 취소 400), Omo 2025-11-25;
Claude는 기본 출력 한도를 넘는 결과를 tool-results 파일로 저장한다(docs_index 288828자);
bearer·grant 파일 경계와 같은 OS 사용자 신뢰 경계; I8 추가 할당 비용(`i8-before-after.md`).
