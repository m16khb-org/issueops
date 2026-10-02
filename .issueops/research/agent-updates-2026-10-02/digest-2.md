# Omo·기타 evidence lanes 검증 요약

조회일: **2026-10-02**. 대상은 `omo-01.md`~`omo-10.md`, `other-01.md`~`other-10.md` 전부다. 구현 제안 없이 원문·설치 코드·IssueOps 코드의 확인 결과와 미해결 조사 항목만 기록한다.

## 결론과 검증 경계

확인된 근거는 실행 복구, 비용을 동반하는 격리, 제한된 관측, 준비 작업 재사용에 관한 것이다. 외부 제품의 기능이나 벤더 측정값을 IssueOps의 결함·성능 개선율로 전환할 근거는 없다. 핵심 5개 주장인 Omo 호스트 격리 비용, Cursor 준비 환경 효과, Cline 체크포인트 실행 시점, Gemini 텔레메트리 기본값, ACP 릴리스 의미를 원문에서 직접 확인했다([S1]~[S5]). 추가 제품 근거와 로컬 결함 후보도 아래에서 구분한다.

**Source fan-out:** 공식 릴리스·공개 소스·배포 메타데이터·설치 코드·IssueOps 코드.
**Claim verification:** 원문에 실제로 적힌 내용과 정적 코드 경로는 확인했다. 런타임 효과·처리량·외부 청구 정확성·프로세스 생존 여부는 실험하지 않았으며 미해결이다.
**Primary-only exceptions:** 기능 계약과 릴리스 내용은 1차 출처만 사용했다. 같은 제작자의 문서·소스·설치물 일치는 독립 검증이 아니다. GitHub API와 npm/PyPI도 발행·배포 근거이지 개선 효과의 독립 재현이 아니다. 벤더 측정값은 그 출처의 주장으로만 유지한다.

## 정정 사항

1. **설치 버전과 실행 엔진을 분리해야 한다.** 모든 Omo 보고서는 당시 `omo-ai 5.1.8 / senpi 2026.10.1-2`를 관측했다. 이번 조회의 `/Users/habin/node_modules/omo-ai/package.json:2-3,19-20`은 **5.1.9 / 2026.10.1-3**이다. 반면 아래 R의 `package.json:2-3`은 여전히 **2026.10.1-2**다. 패키지 업데이트 여부만으로 현재 세션에 새 수정이 적용됐다고 단정할 수 없다. 이 작업에서는 설치하지 않았다.
2. **Gemini 미확정은 해소됐다.** `other-01`과 달리 tag 및 latest API를 이번에 직접 조회했다. latest는 `v0.62.0`, `prerelease: false`, **2026-09-29T21:17:07Z**를 반환했다([S14,S21]; 리드의 `release-corrections.md`와 일치). 조회 시점 최신 정식 릴리스는 v0.62.0이다. v0.63.0 preview의 memory lifecycle 주장을 정식 릴리스로 옮기지 않는다.
3. **LLM 관측 ADR은 현재 구현 증거가 아니다.** `other-06/07`의 근거인 2026-07-02 ADR은 과거 결정이다. 인용한 두 production 경로는 현재 없다. `internal`·`cmd` 검색에서도 `RunExternalLLMPrint` 및 관측 recorder의 production 일치를 찾지 못했다. 따라서 “현재 외부 LLM 사용량을 이미 기록한다”는 주장은 미해결로 낮춘다. 모든 가능한 대체 구현의 부재까지 증명한 것은 아니다.
4. **Cline 문서와 SDK의 차이는 범위 차이일 수 있다.** 현재 SDK는 root agent의 `beforeModel`, `iteration === 1`에서 새 사용자 턴과 저장된 이력을 판정한다([S3], `beforeModel` 구간). 이를 extension의 “매 tool 후” 설명 전체가 틀렸다는 증거로 확대하지 않는다. Cline의 “about one git process”도 전체 snapshot의 프로세스 수 실측이 아니라 릴리스 표현이다([S15]).
5. **ACP 1.10.2는 stable wire v2 선언이 아니다.** 2026-10-01 changelog는 Rust deserialization logging 제거를 기록한다([S5]). migration 문서의 “stable v2 baseline”이라는 표현과 별개로, 같은 문서는 프로토콜 전체를 draft로 명시하고 negotiation·feature flags를 요구한다([S6]).
6. **지속성·취소·병렬 실행을 과대 해석하지 않는다.** monitor dedup은 메모리상의 연속 batch 비교다. file watch는 250ms interval을 사용한다. `parallel()`은 다른 thunk를 끝까지 처리한 뒤 오류를 던지므로 fail-fast가 아니다. Omo orphan PID 신호와 host session close도 모든 descendant의 종료 증명은 아니다(아래 L1~L3).
7. **category 기본값은 실제 사용자 routing이 아니다.** 2026-10-02 리드 추가 지시는 실제 routing이 명시적 **Luna/Sol/Astra overrides**라고 확인했다. `omo-03`의 category constants는 shipped defaults의 근거로만 취급한다. 이번에는 사용자 설정을 새로 읽거나 모델을 실행하지 않았으므로 effective model·fallback 결과는 미해결이다.

## 검증된 findings

### Omo 10개 lane

- **업데이트·격리(01,10):** v5.1.9는 2026-10-02T01:13:10Z에 발행됐으며 in-process child의 timeout/retry 상속, reload 후 tool search, eval approval 수정이 릴리스에 적혀 있다([S1,S13]). 수정 효과는 재현하지 않았다. v5.1.1의 4 parent × 4 child 측정은 격리 후 **2.7GB RSS / 0.8GB footprint**, shared host는 **0.78GB / 0.24GB**였다. prewarm p50/p95는 **1666/3280ms → 1116/1678ms**지만 이미 실행 중인 shared host는 **979/1593ms**였다. 이는 벤더의 제한된 workload 결과이며 격리를 일방적인 최적화로 부를 수 없다([S1]).
- **DAG·routing·capacity(02~04):** dependency가 모두 완료된 node를 admission하며 capacity 신호를 probe 전에 등록한다([S7], `scheduler.ts:577-605,927-930`). amendment는 변경 node와 transitive dependents를 무효화하지만 `load_skills`만 바뀌면 재실행하지 않는다(`manager.ts:166-168,487-508`). category와 explicit model은 batch 상속 후에도 거부한다(`validation.ts:101-109,177-187`). allocator는 eligible lane head 중 오래된 항목을 고르지만 기존 owner resume이 우선하고 overflow 예외가 있다(`concurrency.ts:153-167,217-252`). starvation-free·hard cap 주장은 성립하지 않는다.
- **monitor·MCP(05,06):** 설치 엔진은 설정하지 않은 durable cap을 unlimited로 취급하고 남은 deadline이 있는 ephemeral watch도 복구한다. 공개 terminal 문서는 여전히 “at most **5**”, “never restored”라고 한다([S8], L1). MCP 캐시는 config hash와 7일 TTL을 검사하고 cached lazy server는 연결을 미룬다. 같은 경로의 binary 교체는 hash 입력의 변경이 아니다. 다만 이것만으로 실제 stale schema 사건을 입증하지는 못한다. 검색마다 BM25 index를 만드는 경로도 확인했지만 병목 측정은 없다(L2).
- **context·usage·eval(07~09):** 복구 tracker는 skill 이름과 파일 경로·operation을 복원하며 본문 전체를 복원하지 않는다(L3). 지속 kernel이나 bounded parallel 기능의 존재가 durable storage나 같은 언어의 동시 cell 실행을 의미하지는 않는다. usage의 비용·청구 정확성, RPC telemetry의 실제 전달·누락률은 이 digest에서 미해결이다.

### 기타 10개 lane

- **Gemini(01):** telemetry와 detailed tracing은 기본 off지만 `logPrompts`는 기본 true다. 문서는 latency, tokens, compression, startup, memory, queue depth를 나열한다. exporter overhead나 IssueOps 개선 효과는 제시하지 않는다([S4]).
- **Copilot(02):** v1.0.91 페이지는 2026-10-01과 bounded telemetry shutdown·interrupted busy-state 수정을 기록한다. dynamic workflows는 public preview이며 deterministic/agent steps, pause/resume, 제한을 설명한다. AI-credit 제한은 사후 사용량 보고 때문에 hard ceiling이 아니다([S9,S10]).
- **Cursor(03):** 2026-08-13 Builds는 준비 환경과 last-successful build를 설명하며 **10x boot / 3x first-token**을 내부 결과로 주장한다. 독립 benchmark도 local IssueOps 효과도 아니다([S2]).
- **OpenCode(04):** v1.18.34는 **2026-09-30T22:39:45Z**에 발행됐고 session/parent identity headers와 macOS signing 수정이 확인된다. 속도 수치는 없다([S16]).
- **Aider(05):** v0.86.1 배포물은 **2025-08-13**에 업로드돼 최근 90일 밖이다([S17]). prompt cache는 system/read-only/map/editable prefix를 활용하며 streaming에서 cache 통계·비용을 볼 수 없다고 문서화한다. 이것은 map recomputation이나 IssueOps docs cache 효과의 증거가 아니다([S11]).
- **Cline(06):** SDK v0.0.83은 **2026-09-15T05:53:27Z**에 발행됐다. persistent per-session index와 stat cache 재사용은 릴리스·현재 소스에서 확인된다. 현재 소스의 snapshot event는 outcome·duration·session/run을 기록하지만 tagged introduction date와 실제 delivery는 미해결이다([S3,S15]).
- **Continue(07):** 1.5.47 registry의 `gitHead`에 고정한 소스는 exporter 설정과 enabled 조건을 함께 요구한다. `CONTINUE_METRICS_ENABLED=0`은 비활성화한다. prompt 관련 method의 존재는 OTLP 전송 증거가 아니며 log export에는 TODO가 남아 있다([S12,S18]).
- **OpenHands(08):** benchmarks README는 V0→V1 migration, pinned SDK submodule, Docker/remote workspace를 설명한다. 이것은 현재 runtime의 보안 격리 증명이나 특정 모델 score의 독립 재현이 아니다([S19]).
- **Goose(09):** v1.52.0은 **2026-09-23T14:59:14Z**에 발행됐다. transport 분리, recipe overwrite 방지, extension 없는 chat, recipe consent가 릴리스에 기록돼 있다. latency 개선은 측정되지 않았다([S20]).
- **ACP(10):** 릴리스 숫자·draft 상태·client catalog 포함·실제 conformance를 구분한다. 이 digest는 catalog의 모든 client 유지 상태나 성능을 재확인하지 않았다([S5,S6], `other-10.md`).

## 모든 로컬 gap 후보의 코드 대조

| 후보 묶음·관련 lane | 확인 결과 |
|---|---|
| 일반 visibility 부재: omo-01/03/04/08/09/10, other-01/02/03/07/09 | `internal/domain/issueops/review_metrics.go:14-39,70-98`에 review·완료 phase duration이 있다. `internal/domain/selfaugment/step_stats.go:33-59`는 min/max/mean/p95를 계산한다. `internal/contract/worker/types.go:26-46`에는 queue depth가 있다. Omo-specific occupancy·retry·lineage의 지원 부재는 저장소 전체 결론으로 확정하지 않았다. |
| Omo trace 직접 해석: omo-08, other-10 | `internal/adapter/trace/decode.go:40-82`, `internal/domain/trace/analysis.go:34-58`은 verification·guard·upkeep를 다루며 Omo usage를 해석하지 않는다. 확인 범위는 이 analyzer다. |
| 준비·context 재계산 병목: omo-01/02/06/07/09, other-03/04/05/06/08/09 | `internal/adapter/docs/docs.go:13-38,65-109`에 filesystem 열거·heading read가 있다. 반복 비용·cache 필요성은 측정 전 가설이다. |
| hook telemetry 부재: other-06/07 | `cmd/issueops/hookcli/hook.go:19-43`은 context-only 계약을 명시한다. 고장이 아니라 의도된 경계다. |
| snapshot·verification 부재: other-01/05/08 | `internal/adapter/outbound/sqlstore/maintain.go:16-41`에 WAL checkpoint가 있다. repo snapshot과는 다르다. loop는 verify argv·evidence를 기록한다(`internal/adapter/looprun/lifecycle_test.go:10-44`; runtime 문서의 실행 모드). |
| cancellation·sandbox 부재: omo-10, other-08 | `internal/adapter/hostprobe/process_group_unix.go:14-27`은 group kill을 제공한다. group 밖 프로세스나 hostile-code isolation까지 증명하지 않는다. |
| 설정·관측 재사용: omo-03, other-07 | `internal/application/policy/service.go:37-52`는 매 평가 override를 로드한다. web-fetch DTO에는 duration·latency p50/p95가 이미 있다(`internal/contract/webfetch/types.go:51,117-135`). 외부 LLM recorder는 정정 3의 미해결 상태다. |

## 실패·누락 근거

이번 조회의 원문 fetch는 성공했다. 큰 응답의 잘린 부분은 선택 구간을 다시 표시했다. 직접 확인한 실패는 다음과 같다.

```text
Path not found: /Users/habin/workspace/issueops/internal/adapter/webfetch
ENOENT: no such file or directory, access '/Users/habin/workspace/issueops/internal/core/externalllm/usage.go'
ENOENT: no such file or directory, access '/Users/habin/workspace/issueops/internal/core/external_llm_usage.go'
```

보고서의 과거 실패는 원본 20개 파일에 그대로 남아 있다. 핵심 미해결 근거도 그대로 유지한다: `other-08`의 `runtime_build.py`, `docker_runtime.py`는 **HTTP 404**, `https://docs.swebench.com/`은 **ENOTFOUND**였다. `omo-09`의 release API는 **API-rate-limit error**, `other-09`의 최초 whitespace 명령은 **exit 2** 및 `[: $?: bad number`였다. 이들은 이번 검증의 실패로 바꿔 쓰지 않는다. 런타임 benchmark·restart·process teardown 실험, 테스트·빌드·self-verify는 파일 외 쓰기 금지 범위 때문에 실행하지 않았다.

## 중복 제거한 EXPAND

1. **버전·reload:** 설치 wrapper와 실행 engine의 차이, timeout/retry 상속, tool-search ownership, approval client identity를 확인하는 근거 수집(omo-01/02/06/09).
2. **admission·복구:** frontier wait, owner resume, overflow, skill-only amendment, lost launch와 descendant 종료의 실제 결과를 분리한 조사(omo-02/04/10).
3. **catalog·context 비용:** binary 교체 후 lazy reconnect, schema freshness, 검색 index 비용, docs/skill read 비용, compaction 후 본문 재독 여부의 측정(omo-05~09, other-04/05).
4. **기존 지표 coverage:** review/step/worker/web-fetch와 host event의 대응, 누락·retention·최종 usage 권위·부모/자식 attribution 조사. 오래된 LLM recorder ADR의 현행 여부도 포함한다(omo-08/09, other-01/02/03/06/07/09).
5. **준비 재사용·프로토콜:** 동일 workload의 cold/warm 비용과 결과 비교, pinned OpenHands runtime 근거, ACP draft와 선택 client의 실제 conformance 확인(other-03/06/08/10).

이 항목들은 조사 질문이며 구현 승인이나 변경 권고가 아니다.

## Source index

모두 **2026-10-02 직접 조회**했다. `main` 문서는 mutable이며 조회일을 feature release date로 취급하지 않는다.

[S1]: https://raw.githubusercontent.com/code-yeongyu/oh-my-openagent/v5.1.9/CHANGELOG.md
[S2]: https://cursor.com/changelog/08-13-26
[S3]: https://raw.githubusercontent.com/cline/cline/main/sdk/packages/core/src/hooks/checkpoint-hooks.ts
[S4]: https://geminicli.com/docs/cli/telemetry/
[S5]: https://raw.githubusercontent.com/agentclientprotocol/agent-client-protocol/main/CHANGELOG.md
[S6]: https://raw.githubusercontent.com/agentclientprotocol/agent-client-protocol/main/docs/protocol/v2/migration.mdx
[S7]: https://raw.githubusercontent.com/code-yeongyu/oh-my-openagent/v5.1.8/packages/senpi-task/src/dag/scheduler.ts
[S8]: https://raw.githubusercontent.com/code-yeongyu/senpi/main/packages/coding-agent/docs/terminal-tools.md
[S9]: https://github.com/github/copilot-cli/releases/tag/v1.0.91
[S10]: https://docs.github.com/en/copilot/concepts/agents/dynamic-workflows
[S11]: https://aider.chat/docs/usage/caching.html
[S12]: https://raw.githubusercontent.com/continuedev/continue/d3f60ba9dd3fb5bfd3c91d6fbb41ce1aa768db45/extensions/cli/src/telemetry/telemetryService.ts
[S13]: https://api.github.com/repos/code-yeongyu/oh-my-openagent/releases/tags/v5.1.9
[S14]: https://api.github.com/repos/google-gemini/gemini-cli/releases/tags/v0.62.0
[S15]: https://api.github.com/repos/cline/cline/releases/tags/sdk/sdk/v0.0.83
[S16]: https://api.github.com/repos/anomalyco/opencode/releases/tags/v1.18.34
[S17]: https://pypi.org/pypi/aider-chat/0.86.1/json
[S18]: https://registry.npmjs.org/@continuedev/cli/1.5.47
[S19]: https://raw.githubusercontent.com/OpenHands/benchmarks/main/README.md
[S20]: https://api.github.com/repos/aaif-goose/goose/releases/394774549
[S21]: https://api.github.com/repos/google-gemini/gemini-cli/releases/latest

S7의 형제 경로 `dag/manager.ts`, `manager/concurrency.ts`, `tools/task/validation.ts`, `lifecycle/destroy.ts`도 같은 v5.1.8 tag에서 직접 조회했다.

로컬 anchor prefix **R**: `/Users/habin/.omo/agent/runtime/a1700c8985bbd0c8-ef3ba637dc3d`.

- **L1:** R 아래 `dist/core/extensions/builtin/terminal/monitor-notify.js:60-76,160-169,220-234`, `monitor-file-watch.js:1-33`, `tools/monitor-manifest-binding.js:58-76`, `restore.js:78-85`.
- **L2:** R 아래 `dist/core/extensions/builtin/mcp/catalog-cache.js:6-28`, `config.js:240-258`, `service.js:342-354`, `tool-search/service.js:83-89`.
- **L3:** R 아래 `dist/core/extensions/builtin/compaction/restoration-tracker.js:79-176`, `node_modules/@code-yeongyu/senpi-codemode/src/kernels/js/worker-runtime.js:358-398`.

입력 index: [omo-01](omo-01.md), [02](omo-02.md), [03](omo-03.md), [04](omo-04.md), [05](omo-05.md), [06](omo-06.md), [07](omo-07.md), [08](omo-08.md), [09](omo-09.md), [10](omo-10.md); [other-01](other-01.md), [02](other-02.md), [03](other-03.md), [04](other-04.md), [05](other-05.md), [06](other-06.md), [07](other-07.md), [08](other-08.md), [09](other-09.md), [10](other-10.md).
