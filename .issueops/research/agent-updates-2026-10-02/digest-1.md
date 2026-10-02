# Claude·Codex 근거 검증 요약

확인일: 2026-10-02. 대상: `claude-01.md`부터 `claude-10.md`, `codex-01.md`부터 `codex-10.md`까지 20개 보고서. 구현 제안이나 변경 승인으로 해석하지 않는다.

## 결론과 검증 범위

확인한 릴리스에는 시작 비용, MCP 처리, 세션 복구, 작업 표시 개선이 있다. 그러나 이를 IssueOps의 속도·비용 개선으로 환산할 측정 근거는 보고서에 없다. 로컬에서는 문서 불일치 두 건을 확인했고, SQLite 관측 기능이 없다는 식의 확대 해석은 기존 코드와 맞지 않는다.

**Source fan-out:** Anthropic·OpenAI 원문 문서, GitHub 릴리스 메타데이터, 로컬 구현·설정·운영 문서를 직접 대조했다. 핵심 5개 주장군은 릴리스 사실, 모델 종료 일정, SDK 구조화 출력, MCP 검색·시작 경계, 스트리밍·복구 계약이다.

**Claim verification:** 아래 외부 사실은 모두 공식 단일 출처 계열이다. 벤더 문서와 같은 벤더의 GitHub 기록을 독립 검증 두 건으로 세지 않았다. 로컬 코드는 해당 체크아웃의 사실만 증명한다. 아직 실행하지 않은 동작·효과는 미해결로 구분했다.

**Primary-only exceptions:** 릴리스 날짜·플래그·스키마·지원 상태처럼 공급자가 정의하는 계약은 원문 확인을 조건으로 수록했다. 성능 효과, 실제 과금, 현재 버그 재현에는 이 예외를 적용하지 않았다. 보고서가 독립 출처로 분류한 Agent Skills 사양은 그 독립성을 이번에 확인하지 않았으므로 독립 검증 수에 포함하지 않는다.

## 직접 확인한 주요 사실

1. **릴리스와 날짜.** Claude Code `v2.1.287`은 `2026-10-01T18:00:22Z`, Codex `rust-v0.160.0`은 `2026-10-01T20:19:13Z`에 공개됐고 둘 다 `prerelease:false`다. Claude 기록에는 `prompt_text` 추가와 동일한 마스킹 요구, `message_stop`·compaction·hook 집계 수정이 있다. Codex 기록에는 SQLite 시작·로그 정체 수정, 초기화 오류 노출, plugin manifest 재사용, HTTP 연결 재사용, history pagination, 중복 전송 방지 복구가 있다. 효과의 정량치는 이 릴리스 본문에 없다. 전체 최신 목록을 다시 조회하지 않았으므로 “현재 최신” 대신 “확인한 안정 릴리스”로 한정한다. [S1–S3]

2. **모델 일정.** Anthropic은 `claude-sonnet-4-5-20250929`의 deprecation을 2026-09-30, retirement를 2026-11-30, 대체 모델을 `claude-sonnet-5-5`로 적었다. 적용 대상은 Anthropic 운영 플랫폼이며 Bedrock·Google Cloud 일정은 별도다. `cmd/`, `internal/`, `configs/`에서 해당 모델 ID를 검색했을 때 일치가 없었다. 이는 외부 배포·사용자 설정까지 pin이 없다는 증거가 아니다. [S4]

3. **SDK 구조화 출력.** Claude Agent SDK는 JSON Schema와 `structured_output`을 제공하고 검증 실패 시 재요청한다. 실패 종료와 값 누락을 처리해야 하며, `format`은 annotation으로 받아도 validator가 강제하지 않는다. “스키마 검증”을 사실 정확성이나 모든 JSON Schema 제약 보장으로 확대하지 않는다. [S5]

4. **MCP 경계.** Anthropic API의 `defer_loading`은 모델 context 포함 시점을 바꾸지만 매 요청의 전체 tool definition 전송을 없애지 않는다. Claude Code의 tool search에는 custom `ANTHROPIC_BASE_URL`·`ENABLE_TOOL_SEARCH=false` 등의 예외가 있다. Codex 문서의 optional 초기 catalog grace는 1000ms이며 server startup timeout과 다른 설정이다. IssueOps의 `startup_timeout_sec=30`, `required` 미설정만으로 초기 catalog가 30초 기다린다고 단정할 수 없다. [S7–S9; `internal/adapter/codex/install_config.go:56-74`]

5. **스트림과 세션.** Claude headless는 `stream-json`과 partial-message 옵션을 문서화하고, 종료 시 출력 drain 대기를 최대 30초로 설명한다. 이 값은 모든 출력 처리 중 일반적인 backpressure timeout이 아니다. Codex app-server는 연결별 initialize/initialized와 `turn/completed` 종료를 정의한다. WebSocket overload의 `-32001`을 stdio 전체의 특성으로 확대하지 않는다. [S6, S10]

## 정정 및 로컬 gap 대조

| 보고서·주제 | 원문·코드 확인 결과 |
|---|---|
| Claude `v2.1.286`의 성능 주장 | S14의 release API 본문을 한 번 직접 조회한 결과 다수 allow/deny 규칙의 턴별 overhead와 다수 MCP 도구의 idle CPU 개선 문구는 없어 미해결·요약 제외로 처리하고, 원문에 있는 `--bare` 변경, plugin 설치의 git·folder npm source 거부 및 registry dependency 제한, whole-call retry 상한(기본 최대 14 requests)은 유지한다. |
| Claude-04의 과거 SDK 주장 | `.issueops/research/multi-agent-orchestration-quality-patterns.md:153`의 “structured output validation”이 built-in이 아니라는 문장은 현재 S5와 모순된다. 같은 줄의 다른 항목까지 일괄 정정할 근거는 확보하지 않았다. |
| Claude-10의 catalog 수 | `configs/upstream.json:3-38`은 plugin 4개·Git skill 2개다. `.issueops/operations/hosts.md:94`의 “four plugins and one skill”은 불일치다. |
| 추가 운영 문서 불일치 | `.issueops/operations/hosts.md:104-106`은 install에 `--yes`를 적지만 `internal/adapter/outbound/upstream/claude_plugins.go:53-55`의 실제 인자는 `--scope user`까지다. 설치 실패의 증거는 아니다. |
| Claude-09의 `--bare` | “hook/MCP 증거를 제거한다”는 결론은 과도하다. S6은 ambient discovery를 생략하면서 explicit `--settings`·`--mcp-config`·plugin 인자를 허용한다. 현재 probe도 explicit 설정을 넘긴다(`hostprobe/claude.go:156-175`). 동일 동작 보존 여부는 미실행이다. API-key 요구도 Anthropic API 기준이며 다른 provider credentials 예외가 있다. |
| Codex-02의 app-server 상태 | S10은 **app-server command와 WebSocket transport 둘 다** experimental·production unsupported라고 적는다. 경고를 remote/WebSocket에만 한정하면 지원 범위를 과장한다. |
| catalog 크기·lazy discovery | `internal/adapter/inbound/catalog/mcp/catalog.go:18-45`는 여러 section을 합친 전체 목록을 광고한다. `IssueOpsBasicTools` 한 개를 전체 규모로 세면 안 된다. 전체 schema token·실제 선택 비용은 미측정이다. |
| 실시간 관측·resume | `hostprobe/claude.go:102-129`는 프로세스 반환 뒤 stdout을 해석하고 `:169`에서 persistence를 끈다. `runner.go:258-303`은 수집된 bytes를 검증한다. 이는 probe의 경계이지 저장소 전체에 recoverable workflow가 없다는 증명이 아니다. |
| SDK·OTel·app-server 연동 | `cmd/`, `internal/`, `configs/`의 `app-server`, `claude_agent_sdk`, `ClaudeSDKClient`, 독립 단어 `OTEL`·`otel` 검색은 일치가 없었다. “이 검색 범위에서 직접 연동 anchor가 없다”로 한정한다. Codex template에 OTel block이 없는 것과 사용자 유효 설정은 별개다. |
| SQLite·로그 관측 | `sqlstore/sqlstore.go:340-417`은 wait·hold·contention·outcome을 관측한다. `issueopsrecord/observer.go:18-25,51-92`에는 `wait_ms`, `hold_ms` JSONL과 빠른 무경합 성공 억제가 있다. `cmd/issueops/issueopsapp/issueops_record_store_wiring.go:10-24`에도 observer factory가 있다. 관측 부재보다 실제 wiring coverage·cohort 측정이 미해결이다. |
| history·짧은 duration | `internal/application/issueopsinventory/service.go:22-67`은 `ScanEach`와 scanned/read-error 수를 제공한다. `internal/adapter/trace/decode.go:40-81`은 실패·guard·upkeep을 해석한다. 이 경로만으로 전체 status가 duration을 숨긴다거나 history가 병목이라고 판정하지 않는다. |
| 반복 parsing·connection reuse | `upstream/config.go:16-25`의 file read/JSON decode와 `claude_plugins.go:58-67`의 CLI 실행을 확인했다. `git_skills.go:43-80`의 cache는 materialized skill 저장이다. Codex plugin HTTP pool과 같은 hot path라는 근거는 없다. |
| hook·권한·worktree·concurrency | Codex·Claude installer는 SessionStart만 소유한다(`codex/install_hooks.go:75-89`, `claude/install_hooks.go:81-87`). lease claim은 ID·generation·actor·canonical cwd·token을 검사한다(`internal/application/issueopslease/claim_transaction.go:12-63`). native task list나 worktree를 이 authority와 동일시하지 않는다. 역할·approval·sandbox·instruction-budget의 로컬 효과는 실행 측정이 없는 가설이다. |

표에서 축약한 `sqlstore/`, `issueopsrecord/`, `upstream/`은 `internal/adapter/outbound/`, `hostprobe/`, `codex/`, `claude/`는 `internal/adapter/` 아래다.

## EXPAND: 중복 제거한 조사 항목

아래는 조사 질문이며 구현 제안이 아니다.

- **Context·catalog 예산:** 실제 로드된 instruction bytes, skill metadata/body, 전체 advertised schema, cold/warm MCP 준비 시간을 host·version별로 확인한다. Claude-02/03/10, Codex-04/05/08을 통합했다.
- **지연·비용·위임:** 같은 읽기 작업에서 startup, 첫 event, 종료, tokens/cache, 결과 품질, approval 대기, coordination 비용을 구분한다. Claude-04/05/06/07/08/09, Codex-03/06/07/09의 공통 미측정 항목이다.
- **Exporter 계약:** S11의 Claude beta hook span gate와 S12의 Codex token/MCP metrics를 실제 exporter에서 확인한다. 비용 추정과 billing은 별도다. attribution·exec metric 장애 보고는 현재 버전 재현 전까지 미해결로 둔다.
- **복구·격리:** terminal event, interrupted/resumed history, uncertain submission, shared Git state, ignored-file snapshot 복원 범위를 version-pinned 근거로 확인한다. Claude-08/09, Codex-02/10을 통합했다.
- **State·history 규모:** 기존 span observer의 연결 범위와 wait/hold 분포, retained-record 증가에 따른 scan 비용, config 재읽기 빈도를 확인한다. Codex-01/10의 upstream 수정만으로 로컬 병목을 판정하지 않는다.
- **호환성·근거 보완:** 외부 Sonnet 4.5 pin, plugin activation, experimental config의 tagged source, 실제 concurrency default를 확인한다. live docs를 특정 릴리스에 귀속하지 않는다. Claude-01/10, Codex-04/06/08에 해당한다.

## Source index

모두 2026-10-02 직접 조회했다. 날짜가 없는 live 문서는 확인일만 기록하고 출시일을 추정하지 않았다.

| ID | 원문 | 종류·원문 날짜 |
|---|---|---|
| S1 | https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.287 | 공식 release API, 2026-10-01 |
| S2 | https://api.github.com/repos/openai/codex/releases/tags/rust-v0.160.0 | 공식 release API, 2026-10-01 |
| S3 | https://github.com/openai/codex/releases/tag/rust-v0.160.0 | 공식 release 본문, S2 날짜 |
| S4 | https://platform.claude.com/docs/en/about-claude/model-deprecations | 공식 live docs, 해당 announcement 2026-09-30 |
| S5 | https://platform.claude.com/docs/en/agent-sdk/structured-outputs | 공식 live docs, 발행일 미표기 |
| S6 | https://code.claude.com/docs/en/headless | 공식 live docs, 발행일 미표기 |
| S7 | https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-search-tool | 공식 live API docs, 발행일 미표기 |
| S8 | https://code.claude.com/docs/en/mcp | 공식 live docs, 발행일 미표기 |
| S9 | https://learn.chatgpt.com/codex/extend/mcp | 공식 live docs, 발행일 미표기 |
| S10 | https://developers.openai.com/codex/app-server | 공식 live protocol docs, 발행일 미표기 |
| S11 | https://code.claude.com/docs/en/monitoring-usage | 공식 live telemetry docs, 발행일 미표기 |
| S12 | https://learn.chatgpt.com/codex/config-advanced/ | 공식 live telemetry/config docs, 발행일 미표기 |
| S13 | https://learn.chatgpt.com/codex/config-file/config-reference | 공식 live config docs; experimental mode는 “not currently available” |
| S14 | https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.286 | 공식 release API, 2026-09-30; `--bare` 변경 anchor |

입력 보고서 전체를 읽었다. Claude 01–10은 release, memory, MCP, SDK, delegation, hooks, OTel, isolation, headless, plugins 순이며, Codex 01–10은 release, app-server, exec, context, MCP, delegation, permissions, distribution, OTel, recovery 순이다. 각 파일의 원문 URL·실패 기록은 해당 보고서에 보존돼 있다. 위 요약에 포함하지 않은 세부 수치·기능은 worker 결론만으로 승격하지 않았다.

## 실패·누락 근거와 검증 한계

이번 조회의 Codex release API 출력은 `"[Output truncated: 49.9KB of 353.7KB shown (50.0KB limit). Re-fetch a more specific URL or use web_search for targeted content.]"`로 잘렸고 parsing은 `"JSON Parse error: Property name must be a string literal"`로 실패했다. release 본문과 동일 API의 직접 HTTP 200 JSON 조회로 날짜·stable flag를 복구했다.

잘못 추정한 로컬 경로 조회 실패를 그대로 남긴다:

```text
ENOENT: no such file or directory, access '$REPO_ROOT/internal/adapter/trace/trace.go'
ENOENT: no such file or directory, access '$REPO_ROOT/internal/adapter/hostprobe/process.go'
ENOENT: no such file or directory, access '$REPO_ROOT/internal/adapter/trace/store.go'
ENOENT: no such file or directory, access '$REPO_ROOT/internal/adapter/inbound/hook/context.go'
Skipped missing path(s): $REPO_ROOT/internal/adapter/state
Path not found: $REPO_ROOT/internal/application/hook
```

입력 보고서의 실패 중 `"EISDIR"`, `"ENOENT"`, `"Page not found"`, `"test: $?: bad number"`, `"format: \"json\""` 거부는 이번에 재현한 결과가 아니다. 원문에 기록된 상태로 유지하며, 대체 URL 성공을 과거 실패 삭제로 해석하지 않는다.

추가 검증 한계: Markdown diagnostics는 `"No LSP server configured for extension: .md"`로 실행되지 않았다. 마지막 문단을 읽으려던 첫 요청도 `"Offset 115 is beyond end of file (98 lines total)"`을 반환했고 offset 88로 다시 읽었다.

**Access boundary:** 이번 public-source 조회에는 인증·설치가 필요하지 않았다. 사용자 설정·실제 exporter·배포 pin·benchmark는 조사하지 않았으므로 해당 항목은 미해결이다. 문서 하나만 쓰는 범위라 tests/build·runtime 실험은 실행하지 않는다. 요청한 `git diff --check -- .issueops/research/agent-updates-2026-10-02/digest-1.md`는 exit 0이었다. untracked-aware `--no-index --check /dev/null`도 whitespace diagnostics 없이 차이 존재를 뜻하는 exit 1이었다.
