# 계획: 훅 설치 경로에서 MCP 서비스 재시작 실패 수정

## 문제
`issueops install`이 훅을 등록한 직후 공용 MCP 서비스(`127.0.0.1:47831/mcp`)가 이전 binary로 계속 떠 있어, 새로 설치한 훅과 MCP tool schema가 어긋난다. 사용자가 `issueops mcp service stop`/`start`를 직접 실행해야 한다.

## 변경
1. `issueops hook session-start`가 실행될 때 MCP 서비스의 build id를 확인하고, 현재 binary와 다르면 서비스를 재시작한다.
2. 재시작 실패는 훅 출력의 `systemMessage`로 사용자에게 알리고, 훅은 계속 catalog를 낸다.
3. 재시작 대기 시간은 최대 10초로 둔다.

## 적용되는 결정과 주의사항
- `.issueops/ARCHITECTURE.md`: 대조했으나 없음
- `.issueops/CONVENTIONS.md`: 대조했으나 없음
- `.issueops/TESTING.md`: 새 동작에 테스트를 추가한다.

## 재사용하는 기존 구현
- `internal/adapter/mcpservice`의 `Service.Start`/`Stop`

## 성능 영향
SessionStart마다 build id를 한 번 조회한다. 서비스가 같은 binary면 추가 비용은 HTTP 요청 하나다.

## 하위 호환성과 side effect
훅 JSON 형식은 그대로다. 서비스가 재시작되는 동안 다른 세션의 MCP 호출이 잠깐 끊길 수 있다.

## 수용 기준
- G1: 새 binary 설치 뒤 첫 SessionStart에서 서비스 build id가 새 binary와 같다.
