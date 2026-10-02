# 실제 stdio MCP 목록 관측

## 환경과 방법

- 관측일: 2026-10-02.
- 저장소 HEAD: `02be6d78cb0209c59cd0099614d9a03e453a59b6`.
- 실행 파일: `/Users/habin/workspace/issueops/bin/issueops`.
- 파일 mtime: `2026-10-02T01:18:52.145Z`.
- `./bin/issueops version --json`: exit 0, stdout `issueops 0.1.0`.
- Bun eval에서 `bin/issueops mcp`를 별도 프로세스로 시작하고 stdio로
  `initialize` → `notifications/initialized` → `tools/list`를 순서대로 보냈다.
- request ID별 응답 구독을 먼저 등록하고 각 요청에 10초 제한을 뒀다.
  목록 응답을 받은 뒤 stdin을 닫고 해당 프로세스의 종료를 확인했다.
- lifecycle mutation이나 `tools/call`은 실행하지 않았다.

## 결과

MEASURED: 초기화 응답의 protocolVersion은 `2025-11-25`,
serverInfo는 `issueops` / `0.1.0`이었다.
tools와 resources 모두 `listChanged:true`를 광고했다.

MEASURED: `tools/list`는 **51개 도구**를 반환했다.
도구 배열을 공백 없이 `JSON.stringify`한 UTF-8 크기는 **30,888 bytes**다.
이는 토큰 수가 아니며 JSON-RPC envelope나 다른 MCP 서버의 도구는 포함하지 않는다.

MEASURED: 단일 실행에서 initialize 응답까지 43.47 ms,
tools/list 응답까지 3.15 ms였다. 워밍 상태·시스템 부하를 통제하지 않은 1회
관측이므로 p50/p95, cold-start benchmark, 호스트 전체 지연으로 일반화하지 않는다.
JSON 파싱 오류는 없었다.

| 도구 | inputSchema JSON bytes | description UTF-8 bytes |
|---|---:|---:|
| issueops_execution | 1897 | 252 |
| project_docs_append | 1075 | 230 |
| command_fake_run | 1026 | 148 |
| command_policy_audit | 1026 | 138 |
| command_policy_check | 1026 | 109 |
| gates_check | 986 | 331 |
| project_docs_revise | 922 | 293 |
| api_doc_review | 797 | 274 |
| self_augment_lesson | 702 | 102 |
| self_verify_history | 687 | 149 |
| self_verify | 593 | 143 |
| self_verify_promote | 569 | 144 |

## 코드와 대조

`internal/adapter/inbound/catalog/mcp/catalog.go:18-44`의 `AdvertisedTools`는
여러 capability별 목록을 합친다. `IssueOpsBasicTools` 하나만 읽어 전체
도구 수가 1개라고 결론 내린 초기 분석은 실제 응답과도 모순된다.

## 해석

호스트의 도구 지연 로딩을 검토할 실제 표면은 존재한다.
다만 30,888 bytes가 매 턴 전부 모델 컨텍스트에 들어간다는 사실은 확인하지 않았다.
클라이언트의 deferred loading, 캐시, 이미 선택한 도구 처리에 따라 달라진다.
실제 Claude/Codex/Omo prompt 사용량을 측정하기 전에는 절감량을 제시하지 않는다.

이 관측은 현재 stdio 경로의 정상 초기화·목록 열거를 보여준다.
HTTP로 전환하면 더 빠르거나 더 느리다는 비교 근거는 아니다.
