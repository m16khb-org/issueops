# Issue #41: `issueops update` 뒤에도 MCP 서비스가 이전 build로 응답한다

## 보고 내용
`issueops update`를 실행한 뒤 Claude Code 세션을 새로 열면 MCP tool 목록이 업데이트 전과 같다.
`issueops mcp service status --json`의 `build_id`가 방금 빌드한 `bin/issueops`의 build id와 다르다.
`issueops mcp service stop`과 `start`를 직접 실행하면 그제야 맞아진다. 지난주에만 세 번 겪었다.

## 재현
1. 코드를 바꾸고 `issueops update`를 실행한다(종료 코드 0).
2. 새 Claude Code 세션을 연다.
3. `issueops mcp service status --json` → `build_id`가 이전 값이다.

## 보고자 제안
SessionStart 훅이 세션마다 어차피 실행되니까, 훅에서 서비스 build id를 확인하고 다르면 재시작하면
사용자가 신경 쓸 일이 없을 것 같다. 재시작은 몇 초 걸리니 대기 시간을 10초쯤 두면 될 것이다.

## 완료 조건
`issueops update`(또는 install)를 마친 직후 열린 새 세션에서 MCP 서비스가 새 build로 응답한다.
