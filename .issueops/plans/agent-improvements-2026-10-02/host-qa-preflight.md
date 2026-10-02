# 실제 호스트 검증 준비

## Codex 0.128.0

기존 npm wrapper의 native 파일이 없어서 전역 설치는 그대로 두고, public npm의
동일 버전 darwin-arm64 패키지를 격리 cache로 받았다. 원본 패키지를 보존하고
QA용 복사본만 ad-hoc 서명해 AMFI 실행 거부를 해결했다.

- 실행 파일: `/tmp/issueops-ten-improvements-01a0fa31/codex-runtime/codex`
- 버전 확인: `mon_D7JVP9RM9YEP228H`, `codex-cli 0.128.0`, exit 0
- schema 추출: `mon_83A0AQ51NCNYTC9Z`, exit 0
- schema 위치: `/tmp/issueops-ten-improvements-01a0fa31/codex-schema/ClientRequest.json`

실제 바이너리에서 생성한 schema에는 다음 메서드가 있다.

| 메서드 | 파라미터 |
|---|---|
| initialize | clientInfo 필수, capabilities 선택 |
| mcpServerStatus/list | cursor/detail/limit 선택 |
| mcpServer/tool/call | server/threadId/tool 필수, arguments/_meta 선택 |
| config/mcpServer/reload | 생성 schema 참고 |

app-server는 기본 stdio, `--listen`으로 unix/ws도 지원한다.
`mon_DJVSD688RN69F5F9`에서 격리 HOME의 initialize·MCP 목록·ephemeral thread
생성이 exit 0이었다. 모델 turn이나 인증은 사용하지 않았다.
`mon_4ZB3Y32WA2SR78QR`에서는 실제 HTTP 서버에 연결한 뒤 51개 도구 발견,
`mcpServer/tool/call`로 docs_index 실행, isError=false·ok=true·docs 557개를
검증했다. CLI config override만 사용했으며 전역 Codex 설정은 바꾸지 않았다.

## Claude

기존 `claude --version`: 2.1.287, exit 0.
`mon_9GYK1BAFWYYK9SC2`: Sonnet 5.5의 실제 native 호출에서 MCP connected,
`mcp__issueops__docs_index` tool_use·tool_result, 최종 subtype=success, exit 0.
결과는 ok=true, docs 557개였다.

기본 출력 한도를 넘는 288828-character 응답은 Claude가 자기 tool-results에
저장한다. 초기 QA parser가 이 안내문을 JSON으로 읽어 실패했으며 제품 오류가 아니다.
수정한 QA는 현재 native session의 정확한 artifact 경계만 허용하고 저장된 JSON을
파싱했다. 한도를 올리거나 제품 출력을 줄이지 않았다. 전역 설정·설치·hook은 바꾸지
않았지만 Claude의 정상적인 임시 tool-results runtime 파일은 생성됐다.
`--strict-mcp-config`, 빈 setting sources, built-in tools 비활성화와
docs_index만 허용하는 설정을 사용했다.

## Omo 설치된 native transport

`mon_FCVEKDFGJ059QJT8`, exit 0. 현재 runtime의
`dist/core/extensions/builtin/mcp/transport.js`를 그대로 import해
createMcpTransport/connectMcpTransport, listTools, callTool을 실행했다.
51개 도구, docs_index ok=true·docs 557개, spawnedPID=null을 확인했다.
원래 fetch에 그대로 전달하는 관측 wrapper가 실제 요청에서 2025-11-25를 읽었다.
새 TUI 세션 검증이 아니라 설치된 native transport 구현의 직접 실행 증거다.

## I9 실제 host export

Claude native 호출(`mon_GZGDBARNABFTJXDP`)의 stream-json 7줄을 그대로 저장해
현재 소스 빌드 `trace analyze --input-format claude-stream --json`에 넣었다. exit 0,
complete=true, coverage=complete, 경고 없음. haiku 927/19/0/0 USD 0.001022,
sonnet 4/286/37902/603 USD 0.0128604로 host가 보고한 `modelUsage` 최종 snapshot과
정확히 같고 assistant message usage와 합산하지 않았다. cost_basis는
host_reported_estimate, finality=final, temporality=cumulative다. 원문 session_id는
출력에 없고 SHA-256 digest만 있다.

Omo: `omo --print --mode json --model anthropic-subscription/claude-sonnet-5-5`로 실제
이벤트 스트림 23줄을 받았다(`mon_YEV064BQ8ZPRMQXJ`, exit 0). `omo-json` 분석은 exit 0,
complete=true, assistant `message_end` 1건을 delta·final 샘플로 냈고 2/4/0/53066,
USD 0.132709가 host의 `usage`·`cost.total`과 같다. message에 responseId/id가 없어
contract대로 coverage=unknown이며 합계를 만들지 않았다. session id 원문은 없다.
디스크의 Omo 세션 영속 로그(`type:"message"` 포맷)는 다른 산출물이라 샘플 0,
coverage=unknown, 경고 없음으로 끝났다(계량하지 않음).

Codex exec: 실제 `codex exec --json` export는 확보하지 못했다. 사용자 `~/.codex/config.toml:6`의
`service_tier = "default"`를 0.128.0이 `fast|flex`만 허용해 설정 로드 단계에서 거부한다
(`mon_F04ZN48MF8ZQR3GM`, `mon_JHZT9T0P446GVH39` 모두 exit 1, 출력 0바이트).
`-c service_tier="flex"` override는 파싱 뒤에 적용되어 효과가 없었다. 전역 설정은 수정하지
않았고, 격리 CODEX_HOME에 `auth.json`을 복사·symlink하면 토큰 refresh 때 실제 로그인이
깨질 수 있어 하지 않았다. codex-exec 형식은 결정적 fixture 테스트
(`trace_host_usage_cli_test.go`)로만 검증된 상태다.

## 설치된 HTTP 설정으로 세 host가 같은 서비스 PID에 연결

K가 포함된 바이너리와 J의 installer로 격리 HOME(`z-home2`)에 `install --mcp-transport=http`를 직접
실행했다(`mon_54HFN5VPR2A6R0RA`, exit 0, committed). 세 host 설정이 0600이고 `type:http`·URL·
Authorization 헤더만 있으며 stdio command는 사라졌다. install 출력에 bearer 0건이다.
`inspect --json`은 세 host 모두 transport=http, installed·linked·configured=verified, 뒤 세 관측
not_checked였고 출력에 bearer가 없다.

이어 서비스(launchd test label, PID 42476)를 띄우고 각 host가 **설치된 설정 파일 그대로** 연결했다
(`mon_WFT9BVH72ATAPG93`): Codex 0.128.0 app-server(CODEX_HOME=설치 HOME, override 없음)는 docs_index
ok·docs 587개, Claude 2.1.287은 `--mcp-config=<설치된 .claude.json>`으로 connected·tool_use·success,
Omo 설치 transport는 설치된 `.omo/mcp.json` entry로 51개 도구·docs_index ok·stdio 자식 없음·revision
2025-11-25. 세 실행 전후 서비스 status는 같은 PID 42476이었다.

이 세 실행 artifact로 receipt를 만들어 `inspect --host-receipts`를 확인했다(현재 host 버전은 실제
`--version`, Codex는 QA 사본 경로를 PATH 앞에 둠). 정상 receipt는 세 host discovered·connected
=verified, Omo protocol=verified. Codex·Claude protocol은 협상 revision이 이번 artifact가 아니라 앞선
proxy 실측에만 있어 unknown으로 두었다. connected의 config 해시를 바꾸면 `receipt_stale_config`, 합성
source 파일이면 `receipt_source_synthetic`으로 unknown이 된다. 실험 뒤 서비스 stop, test label 0개,
47831 listener 0개를 확인했다. 실제 사용자 host 설정과 LaunchAgents는 건드리지 않았다.

## 공통 실행 경계

세 호출 모두 같은 foreground 서버 `127.0.0.1:47932`, PID 68137을 사용했다.
서버는 현재 소스를 빌드한 임시 바이너리이며 state도 임시 디렉터리다.
서비스 monitor는 `mon_342V1DVEEDE6BB3B`다.
세 host의 revision은 헤더·initialize 결과만 기록하는 로컬 관측 proxy(127.0.0.1:47933 →
47932, `mon_RRS445NKMGVAZKDK`)로 실측했다.

| host | 관측 |
|---|---|
| Codex 0.128.0 app-server (`mon_3N8TTAH48RDJ9MDD`, exit 0) | `initialize` protocolVersion 2025-06-18 요청·협상, 이후 요청에 `Mcp-Protocol-Version` 헤더 없음, tools/list·resources/list·resources/templates/list·tools/call 200 |
| Claude 2.1.287 (`mon_G1TK2KGQTMHZ711S`, exit 0) | UA `claude-code/2.1.287 (sdk-cli, agent-sdk/0.3.285)`, 모든 요청 헤더 2026-07-28, `initialize` 없이 `server/discover` 200, `notifications/cancelled`는 stateless라 400, resources/list·tools/list·tools/call 200 |
| Omo installed transport (`mon_FCVEKDFGJ059QJT8`) | 헤더 2025-11-25 |

세 host 모두 docs_index ok=true·docs 557개로 결과가 같다.

stdio도 같은 임시 바이너리(`issueops mcp`, env `ISSUEOPS_ROOT`·격리 `ISSUEOPS_STATE_DIR`)로
세 host에서 실행했다. Codex app-server `mon_Z8XMNKRC3KQ0XDM5`, Claude `mon_1JBT9HGAAGMF153A`
(Sonnet native tool_use·tool_result·success), Omo installed transport `mon_1APXF4PE81H87N96`
(자식 PID 생성 확인) 모두 exit 0, 51개 도구, docs_index ok=true·docs 557개로 HTTP와 결과가 같다.
즉 세 host × stdio/HTTP 6경로가 모두 같은 결과를 냈다. Claude의 취소 알림 400은
SDK stateless 의미(세션 없음)이며 호출은 정상 완료됐다. 2024-11-05 등 그 밖의
revision은 실제 host가 요청하지 않아 관측하지 않았다.
이번 증거는 발견·docs_index 연결이다. supervisor 설치, 세 host stdio matrix,
native authorize 뒤 lifecycle mutation, 최종 L schema 결과 검증은 남아 있다.

전역 host 설정·설치는 변경하지 않았다.
