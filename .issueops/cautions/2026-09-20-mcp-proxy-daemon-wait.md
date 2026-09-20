---
name: 2026-09-20-mcp-proxy-daemon-wait
description: Caution record for a solved false case or recurring risk.
---

# 장기 실행 MCP proxy가 시작한 daemon은 Wait로 회수한다

- Date: 2026-09-20
- Kind: `caution`
- Source: cmd/issueops/daemoncli/daemon_start.go; TestStartDaemonProcessReapsChildWithoutBlockingStartup; /tmp/issueops-mcp-fix/qa/reconnect.json
- Summary: Process.Release는 종료된 자식을 회수하지 않아 daemon 재연결을 막는다.
- Context: 설치 후 도그푸딩에서 MCP proxy가 시작한 daemon을 종료하자 child가 zombie로 남았다. signal(0) 생존 판정이 계속 true여서 daemon stop은 refusing to kill unverified daemon process로 실패했고 proxy는 socket_unreachable로 종료했다. 정상 종료와 exit 7 모두 회귀 테스트에서 재현했다.
- Resolution: daemon 시작 함수는 즉시 반환하되 goroutine에서 cmd.Wait를 호출해 자식 종료 상태를 회수한다. 장기 실행 parent에서 Process.Release만 호출하지 않는다. 종료 전후 동일 proxy PID를 유지한 실제 MCP 요청과 daemon generation 변경을 함께 확인한다. 사용자 세션의 이미 닫힌 stdio transport는 별도 host 재연결이 필요하다.
