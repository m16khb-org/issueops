# 열 가지 개선의 항목별 근거 (최종)

이 문서는 최종 보고의 근거 원장이다. 각 칸의 monitor ID와 문서는 parent가 직접 실행하거나 읽은 증거다.
세부 기록은 `parent-verification.md`, 최종 독립 검토는 `final-review.md`, 그 결함 수정 검증은 `final-fixes-verification.md`에 있다.

| # | 개선 | 구현 핵심 | 검증 근거 | 상태 |
|---|---|---|---|---|
| 1 | JSONL 불완전 분석 | Scanner 오류와 손상 행을 `complete:false`와 안전한 warning code로 노출 | 실제 CLI: 70,000자 중간 행이면 `jsonl_scan_error`, 손상 행이면 `invalid_jsonl_line`, 원문은 노출되지 않음. 최종 검토 PASS | 완료 |
| 2 | probe duration·재사용 통계 | 네 probe가 실제 명령 실행 시간을 보존, `Reused`/`ReusedCount`, selfverify contract v7, contract가 다르면 duration 비교 차단 | RED/GREEN `mon_6X3K5VHQTQR7JJEZ`(137/149/163/60ms), `mon_9T9VZR3D0J8C0PMR`→`mon_TNH2QYZAEH3F0ZKX`. 실제 self-verify(격리 사본) `ok:true`, 28단계 실패 0 | 완료 |
| 3 | host 설치·발견·연결·지원 상태 | `HostIntegration` 여섯 관측, schema 1 receipt. config_path·config SHA·host version·transport 신선도 검사와, artifact 호출·응답의 의미 판정(F4) | 빈 HOME은 `failed/not_checked`, http 설치는 `transport=http`. 실패한 docs_index artifact는 unknown이고 실제 artifact는 verified. RED/GREEN `mon_WQ9GGGFZW5ERG8ZQ`→`mon_VQJ1K2RQBJ45RCP0`, F4 검증 PASS | 완료 |
| 4 | structuredContent·outputSchema·annotations | 두 도구만 2020-12 outputSchema와 readOnly/closed-world를 광고하고, 잘못된 출력은 -32603 | `mon_DSTBQ9WYKGD8D67Z`, 실제 stdio·HTTP에서 두 도구만 광고, structuredContent는 text와 같은 object. 최종 검토 PASS | 완료 |
| 5 | SDK·3 host protocol | go-sdk v1.8.0, stdio/HTTP × 2025/2026 | 세 host × stdio/HTTP 6경로가 모두 51개 도구·docs_index 성공. Codex는 2025-06-18, Claude는 2026-07-28, Omo는 2025-11-25로 관측(`host-qa-preflight.md`). 세 host 실제 lifecycle(아래 10)도 통과 | 완료 |
| 6 | 요청 내 Git/history 중복 | history 4회 → 1회, Next 관측을 요청 단위로 | `mon_3JP6ZQABY8ETXERY`, `mon_TN0JJNHY06SMFZ9H`, 실제 `preflight --json`. 최종 검토 PASS | 완료 |
| 7 | metadata 문서 목록 | 상위 64개를 제한 선택, header는 8KiB까지만, 생략 내역 보고 | RED `mon_MGP4CP8B1EDEKHSD`, GREEN `mon_ESQ1RB9R06GH50HT`, 실제 hook에서 13 docs. 최종 검토 PASS(정렬 slice로 heap과 같은 상한) | 완료 |
| 8 | lock 단계 지연 | wait/callback/commit/hold/total, coverage, commit 직전 취소 경계 | `mon_MXCWTN0ZH9RKNKVS`, 변이 RED `mon_M7YY7JZEJBT70B2B`, 동등 workload 비교 `mon_WXRAKNJKH1GCHY5E`(`i8-before-after.md`, 속도 개선 주장은 없음). 최종 검토 PASS | 완료 |
| 9 | host usage·TRACEPARENT | claude-stream/codex-exec/omo-json 입력, strict v00 traceparent, CLI는 1회·HTTP는 요청 header로 바인딩, MCP SQL span observer | `mon_W2KN65X1KWR9VRZ3`, MCP observer RED `mon_ESMGSFWAJG47W0BT`→GREEN `mon_365SCRAGSVMR8XWV`, 실제 Claude·Omo export. 최종 검토 PASS | 완료(Codex exec export는 외부 제약) |
| 10 | 공용 Streamable HTTP MCP·요청 권한 | native `mcp authorize` capability. guard는 grant root의 span과 data write 트랜잭션에서 재검사(F1·F3). 파일 인자는 scope 안으로 제한(F2). launchd/systemd 서비스, http 설치, Omo catalog header | D1 `mon_BSFKHGXB7918X6Y6`→`mon_TAJXM1PVGBQY2SRV`, worker fence `mon_KMWC42DF6C3RATV5`→`mon_57YR1PTV2SZ0N4FB`, F1-F5 수정 검증 PASS. 같은 HTTP 서버(PID 77731)에서 Codex·Claude·Omo가 각각 native authorize 뒤 status→release→재 release 거부→readback을 통과 | 완료 |

## 최종 battery (F1-F5 수정 뒤, parent 직접 실행)

gofmt 출력 없음, `go vet ./...` exit 0, `go test ./... -count=1` 323 packages ok, `go test -race ./... -count=1` exit 0(323 ok, DATA RACE 0),
`go build` exit 0, contractgolden·response golden ok, Python suite exit 0, docs checker 위반은 기존 gitignored evidence 파일의 1건뿐이다.

## 외부 검증 한계

- **Codex exec JSONL export:** 사용자 `~/.codex/config.toml:6`의 `service_tier = "default"`를 Codex 0.128.0이 거부했다. 전역 설정은 바꾸지 않았고 auth 파일도 복사하지 않았다. Codex 경로는 app-server의 MCP 호출과 lifecycle로 검증했다.
- **Codex native 바이너리:** 사용자 npm wrapper의 native 파일이 없다. 같은 버전 공개 패키지를 격리해서 받고, QA용 복사본만 ad-hoc 서명했다.
- **Omo lifecycle:** Omo app-server는 `command/exec`와 `mcpServer/tool/call`을 구현하지 않는다(-32601). authorize는 실제 Omo 세션 프로세스 트리에서 실행했다. MCP 호출은 실행 중인 Omo 런타임의 transport 모듈로 했고, agent loop가 한 호출은 아니다.
- **linux systemd 실제 실행:** 이 장비는 macOS라 단위 테스트(렌더링·파싱)만 있다.
- **기존 성능 테스트:** `TestCheckLargeBodyIsFast`(200ms 단언)는 전체 race 병렬 부하에서 한 번 241ms였고, 단독 race 3회는 0.05-0.06s였다. 이번 작업이 건드리지 않은 패키지라 후속 과제로 남긴다.
