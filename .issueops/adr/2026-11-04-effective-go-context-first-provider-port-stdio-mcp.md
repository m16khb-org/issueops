---
name: 2026-11-04-effective-go-context-first-provider-port-stdio-mcp
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Effective Go 관례 정리: 이름 규칙, context-first provider port, stdio MCP 신호 종료

- Date: 2026-11-04
- Kind: `adr`
- Source: cli
- Summary: Go 소스 이름을 Effective Go 관례에 맞춘다. package clause는 디렉터리 이름과 같고 exported 이름은 패키지 이름을 되풀이하지 않는다. port.IssueProvider와 외부 명령·HTTP 호출은 context를 첫 인자로 받는다. stdio MCP는 SIGINT·SIGTERM에서 처리 중인 요청을 최대 10초 기다린 뒤 취소하고 종료 코드 0으로 끝낸다.
- Context: revive·staticcheck(ST) 측정에서 production 코드에 4,700여 건이 나왔다. package clause가 디렉터리와 다른 패키지 3개, IssueOps 접두어가 붙은 stutter 이름 208개, 대소문자만 다른 이름 쌍, context 유무로 갈라진 provider 이중 API(선택 인터페이스 3개와 fallback), 취소를 보지 않는 외부 명령·HTTP 호출, 신호를 받으면 처리 중인 요청을 버리고 신호 종료 상태로 끝나는 stdio MCP, 문서 주석이 없는 exported 식별자 3,053개가 원인이었다.
- Decision: 1) 이름: package clause를 디렉터리 이름에 맞추고 접두어 stutter를 뗐다. 충돌한 이름은 뜻이 다른 쪽에 New...·...Draft를 더하거나 기존 상수로 합쳤고 옛 이름 alias는 남기지 않았다. 외부 사용이 없는 exported 이름 401개는 비공개로 바꿨다. 2) 문서: exported 식별자와 패키지에 이름으로 시작하는 문서 주석을 둔다. 한국어 주석도 이름 뒤에 공백을 둔다. 3) port: IssueProvider의 CreateIssue·CreatePullRequest·CreateChild·CloseChild와 ReconcilePullRequest가 ctx를 첫 인자로 받고 fallback 인터페이스를 지운다. providerutil.RunContext가 상한 없는 외부 명령에 ctx와 lifetime lease를 붙이고, 시작 실패는 원인을 Unwrap으로 돌려준다. 4) stdio MCP: HTTP 서비스와 같은 종료 방식을 쓴다. 신호 ctx가 끝나면 in-flight 요청을 10초(httpShutdownDrain) 기다린 뒤 요청 ctx를 취소하고, 그래도 닫히지 않으면 2초 뒤 반환한다. 입력 EOF 대기도 같은 ctx에 묶고 첫 신호 뒤 기본 처리를 되돌려 두 번째 신호는 바로 종료한다. 5) revive·ST는 .golangci.yml에 켜지 않고 이슈 폴더의 측정 설정으로만 돌린다.
- Consequences: CLI·MCP 이름, JSON 필드, record schema, golden은 바뀌지 않는다. internal/ 아래 Go 소스 이름은 바뀌므로 이후 변경은 새 이름을 쓴다. stdio issueops mcp 프로세스는 신호 뒤 최대 10초 기다려 종료하고, io update의 고아 MCP 정리는 종료를 기다리지 않으므로 보고 내용은 같다. 두 번째 신호는 즉시 종료한다. revive exported 규칙을 상시 lint 게이트로 켤지는 별도 결정으로 남는다(2026-08-26 이후 문체 linter 비활성 결정과 충돌하므로 새 ADR이 필요하다).
