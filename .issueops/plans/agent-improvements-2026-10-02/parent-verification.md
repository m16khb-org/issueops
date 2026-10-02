# 메인 검증과 후속 수정

## I1

`mon_3JP6ZQABY8ETXERY`에서 trace adapter/domain/application 테스트를 직접 재실행했다.
현재 CLI 빌드는 `mon_35BRHTR40TXVR6Z1`에서 exit 0이었다.
그 바이너리의 `trace analyze --input - --json`에 다음 입력을 직접 전달했다.

| 입력 | exit | complete | findings | warnings |
|---|---:|---|---:|---|
| 정상 sentinel 두 개 | 0 | true | 2 | 없음 |
| sentinel 사이 70,000자 행 | 0 | false | 1 | jsonl_scan_error |
| 손상 행과 정상 sentinel | 0 | false | 1 | invalid_jsonl_line |

손상 행의 private fixture 원문이 출력에 없는 것도 확인했다.

## I2

기존 duration·재사용 패키지 테스트는 `mon_3JP6ZQABY8ETXERY`에서 모두 exit 0.
생산자 범위 밖으로 남았던 두 항목은 메인이 직접 수정했다.

- CLI summary 테스트의 v6 assertion을 `mon_2P29A7C4DSYMFAR1`에서 재현하고
  현재 계약인 v7로 갱신했다.
- `history_compare.go`가 v6/v7 또는 다른 hash의 step duration을 비교하던 결함을
  `mon_9T9VZR3D0J8C0PMR`에서 재현했다. 이제 contract name/version/hash가 같을
  때만 slow-step·p95를 비교하고, 다르면 `step_duration_contract_mismatch`를
  경고한다. 실패 step 등 비시간 회귀 비교는 유지한다.
- `mon_TNH2QYZAEH3F0ZKX`: domain/selfaugment, application/selfverify,
  CLI summary/historycompare 테스트 모두 exit 0. 변경 파일 LSP 오류 없음.

## I7

생산자의 9개 변경 파일을 읽고, 열 수 없는 `.issueops` 디렉터리가 정상 빈 목록으로
취급되는 누락을 발견했다. `TestDiscoverReportsUnreadableDirectory`로
`mon_MGP4CP8B1EDEKHSD` RED를 얻고 `ScanTruncated=true`로 고쳤다.

`mon_ESQ1RB9R06GH50HT`에서 지정된 5개 패키지 테스트, vet, 세 패키지 race,
contractgolden을 직접 재실행해 모두 exit 0을 얻었다.
실제 `hook session-start --json`도 격리 state로 실행해 exit 0,
should_inject=true와 문서 13개를 확인했다.

## 진행 중인 후속 단계

SQL 독립 검토의 CAS/delete·acquisition 경합·취소·span 내 가시성 지적은 보완됐다.
메인은 남은 start-epoch 회귀 사례와 bounded 채널 대기를 추가했다.
`mon_MXCWTN0ZH9RKNKVS` 일반·race 테스트 모두 exit 0이며,
`mon_M7YY7JZEJBT70B2B`에서 start epoch를 제거한 외부 overlay 변이는 실제 실패했다.
coverage complete는 같은 프로세스의 같은 DB 핸들 관측 범위다.
전후 동일 workload 비교도 `mon_WXRAKNJKH1GCHY5E`에서 완료했다.
`i8-before-after.md`에 원시 수치와 추가 할당 비용을 기록했다. 속도 개선을 주장하지 않는다.

I9 host usage ingestion DAG는 완료됐다. 메인은 라벨의 credential/URL 비노출과
잘못된 is_error 타입이 실측 0을 인증하지 않도록 추가 보완했다.
`mon_92FBGSHWQ506X9M5` RED 후 `mon_W2KN65X1KWR9VRZ3`에서 관련 5개 패키지와
4개 패키지 race가 통과했다.

메인이 TraceContext DTO, v00 ParseTraceparent, issueopsrecord.WithTraceparent,
SQL observer의 trace_id/parent_id/trace_flags와 요청 격리 테스트도 추가했다.
빈 값·잘못된 값도 기존 context의 correlation을 지워 서버 환경이 HTTP 요청에
새어 들어가지 않게 한다. bool false는 원문 없는 경고를 출력하라는 신호다.
`mon_38KQP9CCZD7ZX8YT`에서 parser/store 테스트, 동시 요청 race,
두 architecture dependency 검사가 통과했다. CLI 환경과 HTTP header의 진입점
배선은 G의 공용 composition 변경을 받은 뒤 연결해야 한다. I9 전체는 아직 미완료다.

I6는 메인이 preflight/issueops 패키지, Next 요청 경계(`mon_TN0JJNHY06SMFZ9H`)를
재실행하고 실제 `preflight --json` exit 0도 확인했다.

G 권한 구현 DAG `dag_bb3cd76f-c5eb-42e1-8768-9ac661d4936f`는 완료됐다.
메인이 core guard·상태·credential·scope·CLI 조립을 읽고, 검토 D1의 native-only
verifier panic을 `mon_BSFKHGXB7918X6Y6`에서 재현했다. repository가 없는 verifier는
bound/locked capability를 오류로 거부하도록 수정했다. clock만 채워 잘못 허용하지 않는다.
`mon_TAJXM1PVGBQY2SRV`에서 7개 관련 패키지와 authority race가 통과했다.

현재 H DAG는 `dag_051baa6d-0aa9-47b4-aaf4-a6c7472d06cb`다.
Opus가 공유 HTTP, 요청별 scope/authority/context, legacy port의 HTTP parity,
CLI/HTTP TRACEPARENT 배선과 service CLI 표면을 맡고 Fable이 검증한다.
메인은 H 소유 composition·MCP 파일을 동시에 수정하지 않는다.

H 1차 산출물은 Fable 검증에서 조건부 통과였다(`shared-http-verification.md`).
gates `file` 생략 시 서버 cwd 쓰기, gates 절대/`../` 경로의 workspace 이탈,
MCP `BaseSync` 누락으로 completed replace preview 실패, 그리고 persist helper와
loop/worker/state의 `context.Background()`로 인한 commit 직전 취소·guard 재검증
누락이 지적됐다. 메인은 같은 DAG를 amend해 Opus가 다섯 항목을 고치고 Fable이
재검증하도록 했다. H는 완료 처리하지 않았다.

H 교정 증분 producer는 완료됐다. 메인이 워크트리를 직접 확인했다: MCP action 의존성에
`BaseSync: deps.BaseSync`, persist helper 세 함수의 ctx 시그니처, production에서
`WithLock/WithSpan(context.Background())` 0건, gates의 `os.Root` 기반 WorkspaceFileStore와
새 회귀 테스트(`mcp_http_workspace_boundary_test.go`, `request_store_fence_wiring_test.go`,
`execution_persist_context_test.go`, `state_request_context_test.go`). `mon_W8FQ6DR2MMHXN5BQ`,
`mon_YKY37PQJZXR18SEC`에서 mcpcli·gates·adapter/issueops·lease·loop·state·CLI 패키지가 ok,
mcpcli·gates race에 DATA RACE 0건이었고, 실패는 알려진 response golden 하나뿐이다.
Fable 재검증(`verify-http`)은 조건부 통과였다(`shared-http-verification.md`): 교정 5건은 소스·테스트·
변이 RED·실제 바이너리로 확인됐고, 새 결함 1건은 `unboundWorkerStore`가 요청 guard를
pass-through로 덮어써 `worker_run_read_only`가 lock 아래 grant 철회를 확인하지 않는 것이었다.
메인이 `TestScopedWorkerRunLosesGrantRevokedWhileFenceIsHeld`로 RED를 재현하고
(`mon_KMWC42DF6C3RATV5`, err=nil), loop와 같은 `grantFencedWorkerStore`(grant root span 안에서
worker lock)로 바꿔 `newScopedWorkerService()`를 MCP 요청 의존성에 조립했다. worker 디렉터리
`<state>/worker`와 grant root `<state>/issueops_v1`은 다른 DB라 중첩 span 거부에 걸리지 않는다.
`mon_57YR1PTV2SZ0N4FB`: build·vet, Scoped/HTTP/Worker/Trace 테스트, race 모두 exit 0.

J→K→L 단계 DAG `dag_9271bba2-6506-4291-bf76-bccfb8923874`를 시작했다(Opus J, Sonnet K/L,
Fable 독립 검증). J는 supervisor adapter·host HTTP 설치·update 순서, K는 HostIntegration
관측, L은 outputSchema/annotations/structuredContent와 authority 필드의 catalog 이동을 맡는다.
쓰기 범위는 H와 겹치지 않는다.

J producer 완료(`implementation-service-install.md`). 메인 재검증 `mon_ETN589HD1DN96X04`:
mcpservice·installcli·codex·claude·omo·installutil·application/update·updatecli 8개 패키지
`-race` ok, vet exit 0, `bash -n scripts/install-native.sh` 통과, 격리 HOME/state에서
`mcp service status --json`은 stopped·exit 0, `install --dry-run`은 파일 0개와 launchd label 없음
(exit 113). J의 test-unique label은 실제 gui 도메인에서 정리됐고 실제 `~/.claude.json`은
기존 stdio entry 그대로다. 보고의 순서 편차(credential·unit 준비를 build 직후·stop 전에 실행)는
준비 코드가 새 build 안에 있어서이며 stop·교체 전 실패 중단 목적은 지켜진다.

K producer 완료(`implementation-inspect-hosts.md`). 메인 재검증 `mon_ADTPBZ8331BD1SK5`: inspect adapter·
contract·basiccli race ok. 빈 HOME에서 세 host installed/linked/configured=failed, 뒤 세 관측
not_checked. 설치된 HTTP 설정에서는 transport=http·verified였고, 세 host 실제 연결 artifact로 만든
receipt의 verified/stale/synthetic 판정까지 `host-qa-preflight.md`에 기록했다. K 보고의 편차(config hash를
issueops entry 단위로 좁힘, 빌드 유지용 issueopsapp 세 줄)는 근거가 명확하고 Z가 golden에 반영한다.

메인의 launchd 수명주기 직접 확인(`mon_8SGG3BFXGBXEGTQB`, 격리 HOME/state/root, label
`io.issueops.mcp.parentcheck`): 포트 비어 있음 → start running pid 94346, build_id가 실행 파일
SHA-256(`f4f3a960…d516`)과 일치 → launchctl state running·program 경로 일치 → 두 번째 start는
같은 pid(멱등) → bearer/server.json/server.lock/server.log 모두 0600, plist에 Bearer 0건 →
stop stopped, label exit 113, server.json 삭제, 47831 해제. 실제 사용자 LaunchAgents·host 설정은
변경되지 않았다.

M 준비: 메인이 `issueopsrecord.Store.ObserveSpans`(기존 private observe의 공개)와
parent 소유 `issueopsapp/mcp_trace_wiring.go`의 `issueOpsMCPTraceBinding`을 추가했다.
요청 traceparent를 먼저 묶고 observer를 붙여 MCP 경로의 직접 sqlstore span도 관측한다.
`mon_SE1E63RYAY0APREZ`에서 build·issueopsrecord 테스트·vet이 exit 0이었다.
H producer 완료 뒤 `mcp_facade.go`의 BindTrace를 `issueOpsMCPTraceBinding(os.Stderr)`로 바꿨다.
`mon_365SCRAGSVMR8XWV`에서 build, `TestMCPTraceBindingObservesDirectSQLSpansAndClearsCorrelation`
(실제 HTTP dependency로 직접 sqlstore span 3건: 유효 header는 trace_id 포함, 빈 값·잘못된 값은
trace 없이 error 이벤트)과 HTTP trace 격리·CLI observer 테스트, mcpcli 전체, vet이 exit 0이었다
(`mon_ESMGSFWAJG47W0BT`가 RED 재현). 실제 HTTP 서버에서 span 이벤트가 stderr에 찍히는
관측은 오류·경합·느린 span에서만 나오므로 단위 테스트의 실제 dependency 경로로 증명했다.

CLI 실측: 임시 바이너리와 격리 state·임시 git repo에서 `start --new`로 record를 만들고
`record-routing --phase bogus`로 routing store span 안의 오류를 일으켰다.
`TRACEPARENT=00-4bf9…-00f0…-01`이면 stderr의 `issueops_record_span` 이벤트에
trace_id `4bf92f3577b34da6a3ce929d0e0e4736`, parent_id `00f067aa0ba902b7`, trace_flags `01`이
실리고, 잘못된 값이면 `issueops: ignoring invalid TRACEPARENT` 한 줄만 남고 원문은 어디에도
없으며 이벤트에 trace 필드가 없다. 같은 이벤트에서 I8의 acquired/callback_ms/commit_ms/
commit_count/commit_coverage=complete를 확인했다. 성공한 빠른 span은 기존 임계값대로
출력되지 않는다. `phase` 등 observer가 없는 기존 store 경로는 변경 범위 밖이다.

I5 실사용 matrix: 세 host × stdio/HTTP 6경로가 모두 51개 도구·docs_index 557개로 같았고,
Codex 2025-06-18, Claude 2026-07-28(`server/discover`, stateless 취소 알림 400), Omo
2025-11-25를 관측 proxy로 실측했다. I9 실제 export: Claude·Omo는 host 보고값과 일치,
Codex exec는 사용자 설정의 `service_tier="default"` 비호환으로 확보하지 못했다
(`host-qa-preflight.md`).

세 host 실사용: Codex 0.128.0 app-server, Claude 2.1.287 Sonnet native 호출, 설치된
Omo transport가 같은 HTTP 서버 PID 68137의 docs_index를 성공시켰다(`host-qa-preflight.md`).

모델 운영: 사용자 지시로 Codex 계열 사용량이 소진되면 그 역할을 Claude 계열로 옮긴다.
세션 도구 `apply_patch`가 reload 뒤 stale이라 이번 편집은 일치 횟수를 검사하는 정확
문자열 치환으로 적용하고 diff·gofmt·진단으로 확인했다.

공용 MCP authority/service/install DTO·port는 준비했고
`mon_Z1TSNJ200HX9NTCW`, `mon_8H9KGV9TNN38D81J`에서 관련 테스트가 exit 0이었다.
이는 공용 HTTP 서비스 자체의 완료 증거가 아니다.

호스트 검증 환경: Claude Code 2.1.287은 실행된다. 기존 Codex npm wrapper는
0.128.0이지만 darwin-arm64 native 바이너리 누락으로 `codex --version`이 ENOENT다.
전역 설치는 수정하지 않았다. 같은 native 패키지를 npm public registry에서
`--ignore-scripts`와 격리 cache로 받았다(`mon_PH7C6MNSJ3SNENMF`, exit 0).
199699120-byte 바이너리는 `codesign --verify`가 통과하지만 실행은 SIGKILL 137이다.
`mon_VEEQES8K2H78JFFB`에서 AMFI의 unsigned-code 판정을 확인했다.
새 inode에서도 같았다. 원본·전역 설치는 유지하고 QA용 복사본만 ad-hoc 서명하자
`mon_D7JVP9RM9YEP228H`에서 `codex-cli 0.128.0`, exit 0을 확인했다.
검증용 실행 경로는 `/tmp/issueops-ten-improvements-01a0fa31/codex-runtime/codex`다.

I4 parent 재검증(`mon_DSTBQ9WYKGD8D67Z`): catalog/mcp, mcpcli, contractgolden 일반·race 테스트가
통과했다. 새로 빌드한 바이너리의 stdio(2025-11-25)에서 tools/list 51개 중 `docs_index`와
`harness_inspect`만 outputSchema와 `readOnlyHint:true`·`openWorldHint:false`를 광고했다.
`docs_index`의 structuredContent는 text JSON과 키 순서만 다른 같은 object였고(깊은 비교 일치,
docs 557개), `state_list`에는 structuredContent가 없었다. L이 고친 `instance_services_test.go`
fixture는 인스턴스 격리 단언을 그대로 유지한다.

J/K/L 독립 검증 노드는 Fable 슬롯 부족(`No credential slots available`)으로 실패해 같은 run에서
Opus 5.5로 amend했다. 검토 대상에는 root에 `skills/`가 없을 때 install이 root 검증 실패 전에
MCP 서비스를 먼저 시작하는 순서 문제(ok=false, host 설정은 쓰지 않음, 서비스는 남음)를 넣었다.

마무리 지시(사용자): 모두 완료되면 main에 커밋·푸시하고 project docs update, io update까지 진행해
미스테이지 파일 없이 최신화한다. 무관한 변경(Windows 문서 두 개, `rootchat-design-system.zip` 삭제)은
사용자 답변대로 이번 개선과 **분리된 별도 커밋**으로 올린다.

J/K/L 독립 검증(Opus 5.5, `service-inspect-structured-verification.md`)은 세 owner를 conditional로
판정했다. parent가 두 결함을 재현 후 고쳤다.
- D-K1(medium): `/etc/hosts`를 source로 한 위조 receipt가 verified가 됐다. receipt의 `config_path`가 현재
  경로와 같아야 하고 artifact 내용에 `docs_index`(발견·연결) 또는 주장한 revision(protocol)이 있어야
  verified를 유지한다. 실제 host artifact 세 개는 모두 이 조건을 만족했다. RED `mon_WQ9GGGFZW5ERG8ZQ`,
  GREEN·race `mon_VQJ1K2RQBJ45RCP0`.
- D-J1(low): root가 깨진 직접 install이 서비스를 먼저 띄웠다. 서비스 Prepare 전에 host plan dry-run을
  실행한다. RED `mon_V57G8MXKFFC8994F`, GREEN·race `mon_9EYM0V2ECBFVYHVE`.
D-L1(catalog hash 파생 golden·template), D-J2·D-K2 문서 정합, 서비스 env 한계는 Z에 넘겼다.

최종 독립 검토(GPT-6 Astra, `final-review.md`)는 FAIL이었고 F1-F5를 보고했다. 수정 DAG
`dag_a7507601`(F1·F3 Opus 5.5, F2·F4/F5 Sonnet 5.5, 검증 Fable 5.1)가 처리했고, 검증 보고
`final-fixes-verification.md`는 다섯 결함 모두 PASS다. parent가 핵심 설계를 직접 읽었다. guard는
grant root(`issueOpsStateRoot()`)에만 묶이고, 그 root의 Apply·CompareAndApply(Func)는 mutation 적용 뒤
commit 직전 같은 data transaction reader로 grant를 다시 검사한다. F4는 Claude·Codex·Omo artifact의
호출·응답을 짝지어 성공을 판정한다. 수정 뒤 parent battery(`mon_` F1-F5 이후): gofmt 없음, vet 0,
`go test ./...` 323 ok, `go test -race ./...` exit 0·323 ok, build 0.

세 host 실제 lifecycle(같은 격리 HTTP 서버 PID 77731, 각 host가 holder인 lease):
- Codex 0.128.0: app-server `command/exec`로 Codex 프로세스(79861) 안에서 native authorize, Codex
  `mcpServer/tool/call`로 status(active) -> release(released) -> 재 release 거부 -> readback released.
- Claude 2.1.287: 실제 `claude -p`(Opus 5.5)가 Bash로 authorize(세션 프로세스 90662), Claude HTTP MCP로
  같은 네 단계. bearer 노출 없음.
- Omo: 실제 Omo 세션(33293) 프로세스 트리에서 authorize. Omo app-server는 `command/exec`와
  `mcpServer/tool/call`을 구현하지 않아(-32601) MCP 호출은 실행 중인 Omo 런타임의 MCP transport
  모듈로 했다. agent loop가 한 호출은 아니다. 같은 네 단계가 통과했다.
스크립트·로그: `/tmp/issueops-ten-improvements-01a0fa31/hostlc/`.

수정 뒤 전체 race(`mon_7A3SZ9J36Q3XHMT9`): 322개 패키지 ok, `DATA RACE` 0건, 실패 1건.
실패한 `TestCheckLargeBodyIsFast`(`internal/domain/artifactreadability`)는 이번 작업이 건드리지 않은
패키지의 기존 wall-clock 200ms 단언이다. 전체 race 병렬 부하에서 241ms가 걸렸고, 단독 race 3회는
0.05-0.06s로 모두 통과했다(`mon_DYZSG90Q5VBQZ775`). 시간이 검사 대상인 테스트이므로 수정하지 않고
후속 항목으로 남긴다(race 계측 아래 기준선 조정 또는 race build tag 제외 검토). Omo 수정 뒤
`omo`·`architecture` race도 ok다.

Z가 새로 찾은 Omo HTTP catalog cache 키 누락(medium)을 parent가 고쳤다. Omo는 server config 전체
(`hashConfig(normalizeServer(server))`, headers 포함)를 catalog cache 키로 7일 재사용한다. HTTP entry에
`X-Issueops-Mcp-Catalog-Sha256` 헤더로 catalog digest를 넣었다. 서버는 이 헤더를 무시하고, inspect의
config_sha256은 header 값을 가리므로 영향이 없다. RED `mon_4GQ4KG1H230D9023`(digest가 바뀌어도 entry 동일),
GREEN·race `mon_BY9S8DH2W615N2WQ`. inventory 차이는 `Installer.catalogSHA256` 한 줄이다.
`operations/install.md`의 우회 안내를 수정된 동작 설명으로 바꿨다.

Z 이후 parent battery(`mon_DQYX3PJYFNKT7A06`): gofmt 출력 없음, vet exit 0, `go test ./...` exit 0
(323 패키지 ok), contractgolden·response golden ok, build exit 0. 전체 race 첫 실행에서
`TestResponseContractsGolden`이 `calling "tools/call": EOF`로 한 번 실패했다(단독·패키지 race는 통과).
원인은 HEAD부터 있던 테스트 헬퍼 `startHistoryMCPTestSession`의 고정 10초 server context다.
전체 race 부하에서 한 호출이 10초를 넘으면 서버가 끝나 EOF가 난다. 시간 자체는 검사 대상이 아니므로
세션 수명을 `t.Context()`에 묶었다. deadline을 1ms로 줄인 overlay 변이는 실패했고, 수정본은
race에서 통과했다.

중간 battery(통합 전 현재 트리): 격리 venv Python suite `mon_16AM4Q82VTDQ5AM5` exit 0
(root 1·skills 6, 마지막 suite `Ran 27 tests ... OK`), 추적·미추적 Go 파일 gofmt 출력 없음,
`go vet ./...` exit 0(`mon_B1BT76JFK59SGYZP`). Z 통합 뒤 다시 실행한다.

공유 response golden·architecture inventory와 전체 battery는 최종 통합 때 갱신·검증한다.
Windows 관련 다른 작업의 문서 두 개는 수정하지 않았다. commit/push는 하지 않았다.
