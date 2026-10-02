---
name: 2026-10-02-shared-streamable-http-mcp-and-caller-capability
description: Accepted decision record with rationale, alternatives, and consequences.
---

# 공용 Streamable HTTP MCP 서비스와 native 발급 caller capability; 2026-09-23 in-process 결정을 stdio 경로로 좁힌다

- Date: 2026-10-02
- Kind: `adr`
- Source: agent improvements I1-I10 통합 계약(`.issueops/plans/agent-improvements-2026-10-02/contract.md` §1-§3)
- Summary: Codex, Claude Code, Omo는 사용자 계정마다 하나만 뜨는 로컬 Streamable HTTP MCP 서비스(`http://127.0.0.1:47831/mcp`)에 직접 연결한다. 요청의 workspace와 실행 권한은 서버 프로세스 계보가 아니라 native CLI(`issueops mcp authorize`)가 발급한 caller capability 파일로 정한다. stdio(`issueops mcp`)는 호환 표면으로 남는다. 2026-09-23 "issueops mcp serves in-process" 결정은 stdio 경로에 대한 설명으로 범위를 좁힌다.
- Context: 2026-09-23 결정은 daemon이 자기 계보를 actor로 관측하던 결함을 host 세션 안의 stdio 프로세스로 해결했다. 그 대신 세션마다 MCP 프로세스가 하나씩 뜨고, 세 host가 같은 서버를 공유할 수 없었다. 이번 계약은 세 host를 같은 PID의 서버에 연결하면서도 actor 증명을 잃지 않는 방법을 찾았다. 공유 서버에서는 서버 계보가 호출자를 증명하지 못한다. 그래서 호출자를 증명하는 일은 native 세션의 CLI가 맡고, 서버는 그 결과(capability)를 검증만 한다. go-sdk v1.8.0의 stateless Streamable HTTP는 2025 initialize와 2026-07-28 request metadata를 함께 지원한다. 실측에서 Codex 0.128.0은 2025-06-18, Claude Code 2.1.287은 2026-07-28(`server/discover`), Omo는 2025-11-25로 같은 서버에 연결했다.
- Decision:
  - 서버: `issueops mcp --http`가 `127.0.0.1:47831`의 단일 `/mcp`를 `Stateless:true`, `PropagateRequestCancellation:true`, `JSONResponse:false`로 서빙한다(`cmd/issueops/mcpcli/mcp_http.go:20-25,68-70`). 모든 요청은 `<state>/mcp-http/bearer`(0600, 상위 0700)의 256-bit bearer를 요구한다. bearer가 없거나 틀리면 401, Host가 정확한 loopback 주소가 아니거나 Origin이 서비스 origin이 아니면 403이며, request body는 4MiB로 제한한다(`mcp_http.go:93-100`). 고정 포트가 점유돼 있으면 다른 포트로 옮기지 않고 conflict로 실패한다.
  - 수명: darwin LaunchAgent `io.issueops.mcp`, linux systemd user unit `issueops-mcp.service`가 서비스를 감독한다. `issueops mcp service start|stop|status --json`이 supervisor를 제어하고, readiness는 인증된 MCP 응답과 PID·started_at·executable·build_id 일치로 판정한다. supervisor가 없거나 실행되지 않으면 명시적 service 오류를 내고 stdio로 조용히 전환하지 않는다.
  - 설치: darwin/linux에서 `install`/`update`/`bootstrap`의 기본 transport는 http이고, 그 밖의 OS는 stdio다(`cmd/issueops/issueopsapp/install_wiring.go:53-60`). 순서는 host 설정 plan의 dry-run 검증 → (`install-native.sh`·`update` 경로의) build → credential·unit 준비 → 서비스 stop → binary 교체 → start → build_id와 MCP 응답 확인 → host 설정 merge다. 실패하면 host 설정을 HTTP 성공으로 기록하지 않는다. 세 host의 user 설정에는 issueops entry 하나만 `url`+`Authorization` 헤더로 바꿔 넣고 0600으로 쓴다. `--mcp-transport=stdio`는 기존 command 기반 entry를 그대로 설치한다. agy는 transport와 무관하게 stdio다.
  - 권한: 세션은 `issueops mcp authorize`로 workspace 한정 capability를 발급받는다. 이 명령은 CLI가 관측한 native 세션 계보를 확인한 뒤 `<state>/mcp-http/grants/<key>/<sha256(token)>` 경로만 출력하고 token은 출력하지 않는다. HTTP의 workspace 도구는 `authority_file`과 `workspace_root`(필요하면 `cwd`)를 받아 요청마다 scope를 정한다. 서버의 cwd는 어떤 도구에서도 쓰지 않는다. capability는 lease의 generation·claim token·holder 검사를 대체하지 않는다.
  - stdio: `issueops mcp`는 host가 띄운 프로세스 안에서 계속 서빙한다. 2026-09-23 결정의 actor 계보 보존, stdin EOF 뒤 응답 완료 계약은 이 경로에 그대로 적용된다.
- Consequences:
  - 신뢰 경계는 같은 OS 사용자다. bearer와 grant 파일을 읽을 수 있는 같은 사용자의 프로세스는 그 capability를 행사할 수 있다. 이 공격을 막는다는 테스트나 문서 주장은 만들지 않는다(`internal/adapter/outbound/authority/credential_files.go:13-15`). 외부 공개는 지원하지 않는다.
  - unit은 `ISSUEOPS_ROOT`, `ISSUEOPS_STATE_DIR`만 넘긴다(`internal/adapter/mcpservice/supervisor.go:31`). 서비스 프로세스의 `HOME`, `PATH`는 launchd/systemd 기본값이고 `CODEX_HOME`은 없다. 그래서 HTTP의 `harness_inspect`는 기본 Codex 경로를 읽고, PATH에 host 바이너리가 없으면 `host_version_unobservable`을 낸다. env를 넓히려면 테스트와 함께 별도로 결정한다.
  - `harness_inspect`와 `docs_index`만 `outputSchema`와 `readOnlyHint:true`·`openWorldHint:false`를 광고하고, text JSON과 같은 object를 `structuredContent`로 돌려준다. go-sdk는 이 두 도구에 `idempotentHint:false`도 붙인다. SDK가 non-pointer 필드로 직렬화하기 때문이며 issueops catalog가 주장하는 값이 아니다.
  - MCP catalog SHA-256이 바뀌었으므로 stdio 설정의 `ISSUEOPS_MCP_CATALOG_SHA256`과 tracked 템플릿(`configs/omo/mcp.json`, `configs/agy/mcp_config.json`)은 생성기에서 다시 만든다.
  - 2025 revision의 연결 끊김은 2026 취소와 같은 의미가 아니다. Claude는 stateless 서버에 보낸 `notifications/cancelled`에 400을 받지만 호출 자체는 정상 완료된다.
  - 2026-09-23 결정이 남긴 legacy daemon 정리 후속 작업은 그대로 유효하다.
- Evidence:
  - cmd/issueops/mcpcli/mcp_http_test.go: TestHTTPGuardEnforcesBearerHostOriginPathAndBodyLimit, TestHTTPWorkspaceToolsRequireCapabilityAndRejectActorFields, TestHTTPExecutionActionsUseVerifiedCallerWithoutServerAncestry, TestHTTPConcurrentClientsKeepCallerScopeAndTraceIsolated, TestStdioCapabilityBindsCallerWithoutServerAncestry
  - internal/adapter/mcpservice/service_test.go: TestStartLoadsSupervisorAndSecondStartIsIdempotent, TestStartFailsAsConflictWhenPortIsTakenAndNeverLoads, TestUnsupportedOrMissingSupervisorIsAnExplicitServiceError
  - cmd/issueops/installcli/install_mcp_transport_test.go: TestInstallHTTPValidatesTheHostPlanBeforeTouchingTheService
  - cmd/issueops/mcpcli/mcp_transport_test.go: TestRunMCPServesInProcessWithoutADaemon(stdio 경로)
  - `.issueops/plans/agent-improvements-2026-10-02/host-qa-preflight.md`: 세 host × stdio/HTTP 6경로에서 51개 도구와 같은 docs_index 결과, 설치된 HTTP 설정으로 세 host가 같은 서비스 PID에 연결
- Alternatives / rejected options:
  - 세션마다 stdio를 유지한다: 세 host가 서버를 공유하지 못하고 세션 수만큼 프로세스가 뜬다.
  - 세션별 stdio→HTTP proxy를 둔다: proxy가 다시 계보 증명과 build skew 문제를 만든다. 2026-09-23에 걷어낸 구조와 같다.
  - 서버가 peer PID나 요청 헤더로 호출자를 식별한다: 호출자가 보낸 값이 새 신뢰 경계가 된다.
  - 포트 충돌 시 임의 포트로 옮긴다: 세 host 설정의 URL이 어긋나고 어느 서버에 붙었는지 관측할 수 없다.
