---
name: 2026-09-23-issueops-mcp-serves-in-process-the-shared-daemon-leaves-the
description: Accepted decision record with rationale, alternatives, and consequences.
---

# issueops mcp serves in-process; the shared daemon leaves the MCP path

- Date: 2026-09-23
- Kind: `adr`
- Source: architecture review 2026-09-23
- Summary: MCP 요청을 host 세션이 띄운 issueops mcp 프로세스 안에서 처리하고, 공유 daemon은 이전 binary로 떠 있는 proxy만 쓰는 legacy backend로 남긴다.
- Context: daemon 경로에서는 issueops_execution이 os.Getpid()로 daemon 자신의 프로세스 계보를 관측해 native actor 증명이 성립하지 않았다(관측 시점 daemon PPID=1). 그래서 MCP로 호출한 execution mutation은 모든 세션에서 거부되거나, daemon을 띄운 세션이 살아 있으면 그 세션의 receipt만 통과했다. daemon은 권위 상태나 캐시를 갖지 않고(SQLite가 권위다), 세션마다 proxy 프로세스가 이미 떠 있어 프로세스 수도 줄이지 않았다. 2026-07 이후 daemon·proxy fix가 17건이었고 build skew는 cautions/runtime.md 13절의 수동 우회로만 다뤘다. 2026-05-25 ADR은 장기 작업·상태 공유·watch가 실제로 필요해진 뒤 daemon을 두기로 했는데 셋 다 없다.
- Decision: issueops mcp는 항상 in-process로 서빙하고 proxy client 코드를 제거한다. daemon 서버, daemon start/stop/status 명령, daemon_status tool은 이미 떠 있는 옛 proxy가 재연결할 수 있도록 남긴다. update와 bootstrap은 설치 뒤 daemon을 내리기만 하고 다시 띄우지 않는다. 옛 proxy는 재연결하면서 새 binary로 daemon을 띄운다.
- Consequences: 옛 proxy가 모두 사라진 뒤(세션 재시작) daemon 서버, admission, identity handshake, daemon_status tool, update의 daemon stop 단계를 지우는 후속 정리를 한다. 2026-09-23 spike(격리 state, 세션 24개가 100번씩 동시에 state_write)에서 in-process는 p50 0.5ms, p99 73ms, 최대 180ms였고 daemon은 p50 3.3ms, p99 8ms였다. 두 경로 모두 실패 0건이었다. 폭주 부하에서 꼬리 지연만 늘었고 cold start는 in-process 18ms, daemon 29ms였다. in-process stdio는 host가 stdin을 닫아도 이미 받은 요청에 끝까지 응답한다. 예전 proxy가 host EOF 뒤에 지키던 동작이며, 없으면 셸 파이프라인의 요청이 응답 없이 버려진다(TestServeMCPStreamAnswersEveryRequestReadBeforeInputEOF).
- Evidence:
  - cmd/issueops/mcpcli/mcp_transport.go
  - cmd/issueops/mcpcli/mcp_transport_test.go: TestRunMCPServesInProcessWithoutADaemon
  - cmd/issueops/updatecli/update_bootstrap_daemon.go
  - ps -o ppid -p <daemon pid> = 1 (2026-09-23)
- Alternatives / rejected options:
  - daemon을 유지하고 proxy가 peer PID를 넘겨 daemon이 그 계보를 관측한다: proxy가 보낸 PID가 새 신뢰 경계가 되고 daemon 운영 비용은 그대로 남는다
  - ISSUEOPS_MCP_DIRECT opt-in만 유지한다: 기본 경로의 actor 증명 결함이 남는다
  - daemon 서버 코드를 지금 삭제한다: 이미 떠 있는 옛 proxy가 update 뒤 재연결하지 못해 열린 세션의 MCP가 끊긴다
