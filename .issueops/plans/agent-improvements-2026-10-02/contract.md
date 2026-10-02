# 개선 I1-I10 통합 구현 계약

상태: 구현 경계 확정. 이 문서는 조사 노트의 미결정 사항을 대체한다.
기준 HEAD: `92eaa00143841f964c285dacdd41aedbeeecdf2e`.
요청 effort: high. 실제 export: `PI_MODEL=gpt-6-astra`; effort 변수는 export되지 않았다.
이번 산출물은 설계이며 구현·설치·실제 호스트 통과를 주장하지 않는다.

## 1. 결정과 근거

**Decision-complete plan:** Go core, 단일 로컬 HTTP 서비스, native CLI가 발급하는
세션/저장소 한정 caller capability를 채택한다. 세 호스트는 직접 HTTP로 연결한다.
stdio는 호환 표면으로 유지한다. 세션별 proxy, 새 DB, scheduler는 만들지 않는다.

**Repo grounding:** 아래 코드를 직접 읽고 조사 노트와 대조했다.

- `mcpcli/mcp_tool_issueops_execution.go:54-73`은 서버 ancestry를 actor에 넣는다.
  `domain/issueops/native_actor.go:23-43`은 receipt의 ancestry 포함 여부를,
  `application/issueopscycle/native_actor.go:10-19`는 live PID identity를 검사한다.
- `application/issueopslease/claim_transaction.go:24-73`은 generation, cwd,
  claim token과 holder 전이를 검사한다. capability는 이 검사를 대체하지 않는다.
- `adapter/outbound/sqlstore/sqlstore.go:340-413`은 동일 root의 중첩 span을 거부한다.
  `:700-787`의 data commit과 span lock rollback은 서로 다르다.
  `adapter/outbound/issueopsrecord/store.go:135-198`에는 context 없는 Put/Delete가 있다.
- `adapter/trace/decode.go:26-37`은 Scanner 오류를 놓친다.
  `adapter/preflight/preflight.go:51-54`는 history를 네 번 읽는다.
  `domain/selfverify/contract.go:13`은 v6,
  `domain/selfaugment/summary_snapshot.go:13`은 별도 envelope v1이다.
- 설치된 Omo `dist/core/extensions/builtin/mcp/config-schema.js:19-30`,
  `transport.js:154-175`는 `type/http/url/headers`를 지원한다.
  `claude mcp add --help`도 HTTP와 Authorization header를 명시한다.
  Codex [공식 설정 문서](https://developers.openai.com/codex/mcp)의
  `url/http_headers`를 직접 확인했다.
- [SDK v1.8.0 streamable.go](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/mcp/streamable.go)
  `:129-146,210-219,367-437,869-875`를 직접 확인했다.
  stateless는 legacy와 2026을 함께 지원하고 GET/DELETE는 405다.
  2026 disconnect 취소에는 `PropagateRequestCancellation=true`가 필요하다.

위 repo 경로에서 생략한 prefix는 `internal/`, `mcpcli/`만 `cmd/issueops/`다.
입력은 이 폴더의 `brief.md`와 correctness, host-mcp-contracts, request-io,
catalog-read, observability, shared-authority 조사 노트 전부다.

**Assumptions/defaults:** 같은 OS 사용자는 신뢰 경계 안에 있다. capability를
훔친 같은 사용자를 식별한다는 보장은 하지 않는다. 외부 공개는 지원하지 않는다.
**Unresolved questions:** 구현을 막는 설계 질문은 없다. 실제 호스트 revision과
성능 수치는 아래 acceptance에서 측정하며, 실패를 지원 확인으로 바꾸지 않는다.

## 2. I10 요청 권한

### 발급과 전달

`issueops mcp authorize --workspace-root PATH ACTOR_FLAGS --json`을 native 세션에서
한 번 실행한다. CLI가 직접 관측한 ancestry, PID/start-time/executable을 기존
native/live 검사로 검증하고 즉시 종료한다. caller가 보낸 ancestry는 받지 않는다.
cycle/holder 존재는 요구하지 않는다. 권한 발급은 lease를 변경하지 않는다.
발급 scope는 canonical source root와 Git common-dir identity다. 동일 저장소의
worktree만 포함하며 다른 repo는 별도 발급해야 한다. 비Git 디렉터리는 common-dir을
비우고 canonical source root 내부로만 scope를 제한한다.

키는 SHA-256(scope-kind, common-dir 또는 비Git source-root,
정규화한 host/session_id/agent_id의 NUL 구분 문자열)이다.
32 random bytes를 base64url로 만든 secret과 키를 `key.secret`으로 합친다.
동일 키 재발급은 이전 secret을 철회한다. TTL은 12시간이다.
새 bucket `issueops_authority_v1`에는 actor identity/process receipt, scope, hash,
발급·만료 시각만 저장한다. ancestry는 발급 순간 검증하고 저장·재생하지 않는다.
기존 lease/schema v1은 바꾸지 않는다.

CLI는 raw token을 stdout에 내보내지 않는다.
`<state>/mcp-http/grants/<key>/<sha256(token)>`에 immutable 0600 파일을 만들고
`{ok,authority_file,expires_at}`만 반환한다. 재발급은 새 경로를 반환한다.
workspace-bound MCP 도구에 `authority_file?:string`을 공통으로 추가한다.
서버는 이 managed directory의 정확한 key/digest 파일만 root-relative open으로 읽고
symlink·비정규 파일·다른 uid·group/world permission을 거부한다. 임의 파일을 읽지 않는다.
경로를 아는 같은 OS 사용자는 capability를 행사할 수 있다는 경계를 명시한다.
raw token, 경로, ancestry는 tool 결과·audit·trace·SDK logger에서 제외한다.

HTTP workspace-bound 호출은 capability가 필수다. actor 관련 입력
`host/session_id/agent_id/session_pid/session_started_at/session_executable`과
capability를 함께 보내면 거부한다. stdio는 capability 또는 기존 native ancestry 중
하나를 쓴다. 공유 HTTP에서 prepare/claim/release/resume/replace의 모든 action,
reseed/takeover/reconcile/complete를 그대로 제공한다. grant는 **신원 증명**이며
lease 소유권이 아니다. claim token, holder, generation/expected_generation,
completion_generation, cwd, confirm, fingerprint, CAS는 기존 core가 계속 판정한다.
generation을 grant에 고정하지 않으므로 여러 단계의 prepare/replace 중
자신이 만든 generation 변경으로 후속 commit이 막히지 않는다.

**필수 action parity:** `application/issueopsexecution/execution_api.go`의
prepare/status/claim/release/replace/resume/reconcile/complete 전체와 replace의
하위 action을 기본 HTTP에서도 dispatch한다. active-holder-only 발급이나
`native_bootstrap_required`로 특정 lifecycle action을 CLI/stdio에 돌리는 구현은
수용하지 않는다. native bootstrap은 caller identity 발급 한 번일 뿐이다.
추가 lease-specific grant를 도입하더라도 이 pre-lease caller 경로를 대체할 수 없다.
claim token 검증·소비, generation/CAS, canonical workspace 검증은 그대로 남긴다.
회귀 테스트는 CLI/stdio와 HTTP의 action 집합을 대조하고, bootstrap 뒤 lease 없는
prepare/claim 및 holder 변경 fixture에서 core의 허용·거부 결과가 같은지 확인한다.

### 정확한 공용 인터페이스

새 `internal/contract/authority` DTO다. state/receipt는 snake_case JSON,
Use는 내부 전용이며 token은 `json:"-"`이다. `model`은 contract/issueops다.

```go
type Record struct {
    SchemaVersion int
    Key, SourceRoot, GitCommonDir string
    Actor model.NativeActor
    TokenSHA256, IssuedAt, ExpiresAt string
}
type IssueRequest struct {
    WorkspaceRoot string
    Actor model.NativeActor
}
type Receipt struct { OK bool; AuthorityFile, ExpiresAt string }
type Use struct {
    Key, Token, WorkspaceRoot, CWD, Tool, Action string
}
type Scope struct { WorkspaceRoot, CWD, SourceRoot, GitCommonDir string }
```

`model.VerifiedActor{Identity model.NativeActor; Method string}`는 내부 검증 결과다.
Method는 native_ancestry/capability이며 JSON 입력·state로 deserialize하지 않는다.
capability 결과의 ProcessAncestry는 항상 nil이다. 서버 ancestry나 저장된 ancestry를
주입하지 않는다. 신원을 검사하는 것과 관측한 계보를 주장하는 것을 분리한다.

`internal/port/authority`의 정확한 경계:

```go
type RecordReader interface {
    Get(bucket, id string) ([]byte, bool, error)
}
type Repository interface {
    Within(ctx context.Context, key string,
        fn func(*authority.Record) (*authority.Record, error)) error
}
type ActorVerifier interface {
    Verify(context.Context, model.NativeActor) (model.VerifiedActor, error)
}
type ProcessInspector interface {
    Inspect(context.Context, model.NativeProcessReceipt) (
        string, model.NativeProcessReceipt, error)
}
type CredentialFiles interface {
    Write(ctx context.Context, key, token string) (path string, err error)
    Read(ctx context.Context, path string) (key, token string, err error)
}
```

`Within`은 기존 state root span 안에서 grant를 읽고 callback의 non-nil 반환만 Apply한다.
`internal/application/authority.Service` 메서드는
`Issue(context.Context, authority.IssueRequest) (authority.Receipt,error)`,
`Bind(context.Context, authority.Use) (context.Context,model.VerifiedActor,error)`,
`BindSpan(context.Context,authorityport.RecordReader) (context.Context,error)`,
`Verify(context.Context,model.NativeActor) (model.VerifiedActor,error)`,
`Check(context.Context, authority.Use, authorityport.RecordReader) error`다.
생성자는 `New(Repository,ProcessInspector,CredentialFiles,func()time.Time,ScopeResolver,io.Reader) *Service`;
entropy는 crypto/rand.Reader다. Issue는 native 분기만 허용한다.
Within callback에서 immutable credential 파일을 만들고 grant를 Apply한다.
중간 crash/commit 실패의 고아 파일은 검증에서 거부된다. 재발급으로 복구한다.
공개 오류는 authority_required/authority_invalid, schema 오류는 기존 invalid state다.

Bind는 hash/TTL/scope/live receipt를 확인한 뒤 private context key에 Use를 보관한다.
Verify는 이 context가 있으면 매번 grant를 검증하고, 없으면 기존 native ancestry+
live 검사를 한다. HTTP에서는 capability와 actor 입력의 혼합을 거부한다.
stdio에서는 둘을 함께 받을 수 있지만 supplied actor가 검증한 identity와 일치해야 한다.
clientInfo/session_id/MCP session ID나 JSON의 verified=true는 증거가 아니다.

G는 lease claim/release/resume/reseed와 completion의 ProcessInspector 의존 자리를
ActorVerifier로 바꾸고, reconcile/replacement/sync-base/preparation/publication의
NormalizeNativeActor 호출도 같은 verifier로 연결한다. native helper는 bootstrap용으로
남긴다. domain은 shape 정규화와 권한 판단만 수행하고 HTTP에 ancestry를 요구하지 않는다.

일반 mutation도 빠뜨리지 않는다. `MutationAuthority.Authorize(ctx,record,actor)`가
Verifier를 호출하고 `domain/issueopsauthorization.ValidateHolder`의 세 번째 인자인
`*issueopsauthorizationcontract.VerifiedActor`에 결과를 전달한다.
Z는 `contract/issueopsauthorization/types.go`에
`type VerifiedActor = issueopscontract.VerifiedActor`를 추가한다.
domain은 기존 capability-local contract import를 유지하며 architecture 예외를 추가하지 않는다.
holder identity와 PID receipt를 verified.Identity와 대조한다.
`NewMutationAuthority(pathsMatch,verifier)`와
`AuthorizeHolder(ctx,record,actor,pathsMatch,verifier)`로 signature를 고정한다.
기존 `MutationAuthority.Validate`도 `Validate(ctx context.Context,record model.IssueOpsRecord,actor *model.IssueOpsActor) error`
로 변경한다. execution 없는 record의 기존 무소유권 경로는 보존하고, HTTP ingress의
scope 인증과 혼동하지 않는다. 기존 signature를 감싸는 호환 메서드는 추가하지 않는다.
G가 모든 호출자와 인접 테스트를 같은 단계에서 갱신한다. 특히 application의
`issueopsbranch/{link,prepare,retarget,workspace_link}.go`,
`issueopsdelegation/{start,status,verdict}.go`,
`issueopscycle/plan_link_authority.go`, `issueopsexecution/execution_reconcile.go`와
`cmd/issueops/issueopsapp/issueops_{artifact_verification,body_sync,branch_prepare,branch_retarget,child_create,child_start,link,publication,review_reflection}_wiring.go`
9개는 Z-integration까지 미루지 않고 G가 소유한다.
G는 LSP references로 추가 호출자도 확인하고 migration을 끝낸 뒤 H에 넘긴다.
native 일반 mutation은 actor의 관측 ancestry와 record holder의 process receipt로
검증 입력을 만들고, HTTP는 Bind 결과를 쓴다. 검증 전 identity 대조를 건너뛰지 않는다.
lease/completion의 내부 resolveActor도 verified.Identity를 domain actor로 변환한다.
이는 기존 native/live guard의 검증 경로 대체이며 guard 삭제가 아니다.

### 원자성·철회

sqlstore에
`WithRecordGuard(ctx context.Context, bind func(context.Context,authorityport.RecordReader)(context.Context,error)) context.Context`
를 추가한다. DB.WithSpan이 lock 획득 후 BindSpan을 호출하고 반환된 context로 callback을
실행한다. BindSpan은 Check로 재검증하고 reader를 private context key로 전달한다.
span 안의 Verify는 이 reader를 재사용하고 중첩 span을 열지 않는다.
grant는 기존 state root DB의 `issueops_authority_v1` bucket에 저장한다.
Issue의 Within은 lease span 밖에서만 열며 Bind/Verify/BindSpan은 자체 span을 열지 않는다.
Record.Actor의 ProcessAncestry는 encode 전에 비우고 저장 후 읽기 테스트에서 nil을 확인한다.
TTL은 BindSpan이 context에 저장한 진입 시각 기준이며 rotation과 write는 같은 lock으로 직렬화한다.
I8이 context 없는 Store Put/Delete를 Apply로 바꾼다. lifecycle의 holder/generation/CAS는
각 기존 transaction에서 별도로 검사한다. release 뒤 token이 살아 있어도 소유권은 없다.
native 재발급은 만료/철회/host 재시작 때만 필요하며 generation 변경 때문에는 필요 없다.
서버 restart는 grant를 보존하고 host restart/PID 재사용은 거부한다.
commit 직전 취소를 확인하고 commit 후 취소는 rollback하지 않는다. 외부 side effect는
기존 intent/finalize/reconcile을 유지하며 응답 유실은 status로 확인한다.

### 모든 도구 family의 workspace 경계

공용 MCP catalog에 `authority_file/workspace_root/cwd`를 선택 필드로 추가한다.
workspace 호출에는 capability와 해석된 scope가 필수다. root는 명시적 workspace_root, 기존 repo/path,
또는 조회한 record의 root에서만 결정한다. 복수 입력이 충돌하면 거부한다.
cwd 생략은 검증된 요청 root로만 채운다. 상대 파일 경로는 그 cwd에 대해 해석하고
root 밖 접근은 기존 policy로 판정한다. `os.Chdir`, process Getwd/env fallback,
전역 current workspace/actor는 금지한다.

| Dispatch family | HTTP root 결정 |
|---|---|
| project | preflight는 path, project_docs/API-doc은 repo 또는 workspace_root. harness_inspect의 repo 없는 호출, docs_index/skill_manifest/commit_policy는 설치 root |
| issueops | ID의 record.Repo/source root, 실행 cwd는 record canonical worktree와 기존 action 규칙 대조. 최초 생성은 명시적 repo. grant common-dir도 대조 |
| gates | 모든 init/check/status/report/abandon에 workspace_root. file/files는 요청 cwd 기준 |
| loop | start의 repo 필수. 나머지는 loop record.Repo 조회 후 grant scope 대조 |
| policy_state | command_policy/check/audit/fake_run은 workspace_root와 그 안의 cwd. state_*는 고정 user-state namespace이며 cwd를 사용하지 않음 |
| assistant_worker | commit/lint는 repo, worker_run_read_only는 workspace_root/cwd. worker queue/status/list/cancel은 고정 worker state. fetch/daemon/contract는 cwd 독립 |
| channel | 고정 user-state channel namespace, cwd 독립. 새 workspace 격리 의미를 만들지 않음 |
| self_loop | self_verify/self_augment와 history/candidate는 명시된 설치 root+user-state. 사용자의 repo로 암묵 전환하지 않음 |

설치 root/user-state 도구는 bearer만 요구하고 공용 SDK server의 immutable dependencies를
쓴다. workspace 도구는 scope 검증 후 request-local dependencies를 만든다.
공통 schema 등록 시 각 도구를 위 둘 중 하나로 명시하고 미분류 도구는 HTTP 등록을
실패시킨다. annotation으로 권한 분류를 추정하지 않는다.

`port/authority.ScopeResolver.Resolve(ctx context.Context,workspaceRoot,cwd string)(authority.Scope,error)`를 추가한다.
`MCPDependencies.RequestScope func(context.Context,string,map[string]any)(authority.Scope,error)`,
`ForRequest func(context.Context,authority.Scope,model.VerifiedActor)(context.Context,MCPDependencies,error)`
를 확정한다. H의 handler는 RequestScope→CredentialFiles.Read→Bind→ForRequest→dispatch
순서로 처리한다. ForRequest가 guard를 붙인 context와 요청 actor를 모든 mutation closure에
전달하고 dispatch도 반환된 context를 쓴다.
`resolveHandlerGroup`의 반환 타입은 `func(context.Context,MCPToolCall) MCPToolOutcome`다.
HTTP의 DefaultTarget/Getwd/CurrentDirectory/guard.BaseDir/APIDoc/projectdocs/cleanup
의존성은 scope의 값으로만 생성한다. context.Background로 취소·권한 context를 버리지 않는다.

## 3. I5/I10 transport·설치·수명

SDK를 **v1.8.0**으로 고정한다. 기존 checksum 검증과 `go mod tidy`는 SDK owner만 한다.
단일 `/mcp`, `127.0.0.1:47831`, `Stateless:true`,
`PropagateRequestCancellation:true`, `JSONResponse:false`를 쓴다.
MCPGODEBUG compatibility override는 사용하지 않는다. 2025 initialize/fallback과
2026 request metadata 검사는 SDK가 맡는다. 2025 disconnect를 2026 취소와 같다고
주장하지 않는다. stdio의 initialize/취소 계약도 SDK대로 보존한다.

모든 HTTP 요청은 설치 시 생성한 256-bit bearer를 요구한다. 인증 실패는 401,
잘못된 Host/Origin은 403이다. Host는 정확한 loopback address:port만 허용한다.
Origin은 없거나 정확한 서비스 origin이어야 한다. request body cap은 4MiB다.
bearer는 `<state>/mcp-http/bearer` 0600, 상위 디렉터리는 0700이다.
로그는 method/tool/outcome/duration allowlist만 사용한다.

세 host user config는 기존 issueops entry만 merge한다:

```toml
# $CODEX_HOME/config.toml, 기본 $HOME/.codex/config.toml
[mcp_servers.issueops]
url = "http://127.0.0.1:47831/mcp"
http_headers = { Authorization = "Bearer <installer-supplied-secret>" }
```

Claude `$HOME/.claude.json`과 Omo `$HOME/.omo/mcp.json`:
`mcpServers.issueops={type:"http",url:"http://127.0.0.1:47831/mcp",headers:{Authorization:"Bearer ..."}}`.
실제 secret은 installer만 주입하고 config는 0600으로 쓴다.
command/args/env 기반 stdio entry는 제거하며 별도 proxy를 넣지 않는다.
명시적 project-local config도 같은 endpoint를 사용하지만 secret을 저장하지 않는다.
해당 host의 user entry를 사용하고 중복 project entry는 생성하지 않는다.
agy는 stdio를 유지한다. `--mcp-transport=stdio`로 세 host의 기존 설치도 선택 가능하다.
기본값은 darwin/linux에서 http, 그 밖에서는 stdio다.
`port.NativeInstallRequest`에 `MCPTransport,MCPURL string`,
`MCPBearer string`을 추가하되 bearer는 `json:"-"`다.
새 `contract/mcpservice.Status`는 `OK bool; Status string; PID int;
BuildID,URL,ErrorCode string`이며 snake_case JSON이다.
`port.MCPService`는 `Start(context.Context)(Status,error)`,
`Stop(context.Context)(Status,error)`, `Status(context.Context)(Status,error)`다.
J가 구현하고 update/bootstrap은 이 port만 호출한다.

`issueops mcp service start|stop|status --json`, foreground는 `issueops mcp --http`.
서비스 DTO는 `{ok,status,pid,build_id,url,error_code}`이며
status는 running/stopped/stale/conflict다. 비밀은 반환하지 않는다.
(2026-10-03 Z 보완, D-J2) supervisor·서비스 state·서비스 binary를 관측하지 못하면 status는 빈 문자열,
ok는 false이고 error_code가 원인을 나타낸다: `supervisor_unsupported`, `supervisor_unavailable`,
`state_unreadable`, start 때 binary를 읽지 못한 `build_mismatch`
(`internal/adapter/mcpservice/service.go:114,117,138,305`). 네 상태 중 하나를 추측해 채우지 않는다.
고정 포트 충돌 시 임의 포트로 바꾸지 않고 conflict로 실패한다.
`<state>/mcp-http/`의 OS file lock으로 단일 인스턴스를 보장하고
PID+started_at+executable+build_id가 맞을 때만 stop한다.

darwin LaunchAgent `io.issueops.mcp`, linux systemd user `issueops-mcp.service`를
설치한다. start/stop은 supervisor를 제어하고 readiness는 인증된 MCP 호출로 확인한다.
unit 경로는 `$HOME/Library/LaunchAgents/io.issueops.mcp.plist`와
`$HOME/.config/systemd/user/issueops-mcp.service`다. Exec은 절대 binary 경로와
`mcp --http`, 명시적 ISSUEOPS_ROOT/ISSUEOPS_STATE_DIR만 사용한다.
supervisor 미지원/미실행은 명시적 service 오류이며 silent stdio 전환은 하지 않는다.
SIGTERM 시 listener를 닫고 10초 Shutdown drain, 이후 context 취소 후 종료한다.
install/bootstrap/update는 credential/unit 준비 → build → service stop → binary 교체 → service start →
build_id와 MCP 응답 확인 → host config merge 순서다. 실패하면 HTTP 설정 성공으로
기록하지 않는다. dry-run은 파일·프로세스·credential을 만들지 않는다.
검증은 격리 HOME/state에서 먼저 수행한다.

## 4. I1-I9 구현 계약과 개별 acceptance

### I1 / I9: trace와 usage

I1은 기존 issueops Scanner cap을 유지한다. malformed nonblank 행 또는 scan 오류면
`TraceAnalyzeResult.Complete bool`의 `complete=false`, 안전한 reason code를
warnings에 추가한다. 정상 finding이 있어도 경고를 지우지 않는다. `ok`는 분석 실행
성공이다. JSONL fallback 자체는 손실이 아니다.

I9는 `TraceAnalyzeRequest.InputFormat string`을 추가한다.
CLI `--input-format=issueops|claude-stream|codex-exec|omo-json`, 기본 issueops다.
host 입력은 file/stdin만, 16MiB/file·1MiB/line cap이다.
초과/잘림은 complete=false이며 원문 parser 오류를 출력하지 않는다.
M은 application/trace.Effects를
`Load(input,format string)(source string,body []byte,truncated bool,err error)`와
`Decode(body []byte,format string) tracedomain.Input`으로 바꾼다.
Input에 `Incomplete bool; Warnings []string; Usage *tracecontract.UsageReport`,
Analysis에도 `Incomplete bool`을 추가한다. A는 먼저 Incomplete/Warnings만 도입한다.
Load는 host 형식에서 cap+1까지만 읽고 truncated를 반환한다.

`TraceAnalyzeResult.Usage *UsageReport`를 추가한다:
`UsageReport{Samples []UsageSample; Coverage string; Warnings []string}`;
`UsageSample{Host,Version,ScopeID,SessionID,TurnID,MessageID,Provider,Model,Finality,Temporality,Epoch,CostBasis string; InputTokens,OutputTokens,CacheReadTokens,CacheWriteTokens *int64; CostUSD *float64}`.
JSON은 snake_case, nil은 null이다. ID는 SHA-256 digest로만 반환한다.
숫자는 finite/nonnegative만 수용한다. prompt/text/thinking/tool payload는 버린다.

Claude result.modelUsage는 scope/model/epoch의 마지막 cumulative snapshot만 보존한다.
main-loop usage와 합산하지 않는다. Codex turn.completed.usage는 delta,
Omo assistant message_end.message.usage는 final authority다. update/turn_end/agent_end를
다시 더하지 않는다. 안정 ID가 없으면 동일 숫자의 별개 turn을 합치지 않도록
sample을 보존하되 coverage=unknown으로 두고 합계를 만들지 않는다. 첫 cumulative는 delta가 아니다.
reset/감소/충돌은 경고하고 추정 delta를 만들지 않는다. 누락과 crash zero는 unknown,
명시적으로 성공한 final의 0만 measured다. cache/reasoning subset을 total에 더하지 않는다.
coverage는 complete/partial/unknown, finality는 final/partial/unknown,
temporality는 delta/cumulative, cost_basis는 host_reported_estimate/unknown이다.
scope는 단일 입력 export 경계이며 reset마다 epoch를 증가시킨다.
이미 parent cumulative에 포함된 child sample은 별도 합산하지 않는다.
Claude의 inputTokens/outputTokens/cacheReadInputTokens/cacheCreationInputTokens,
Codex의 input_tokens/output_tokens/cached_input_tokens,
Omo의 input/output/cacheRead/cacheWrite만 각 nullable token 필드에 매핑한다.
host가 알려준 pricing basis가 없거나 unknown이면 cost는 null이다.
모르는 event/field는 계량하지 않고 unknown coverage로 남긴다.
원본 ID는 256 bytes까지만 받아 digest로 바꾸고 초과하면 unknown으로 처리한다.

`contract/trace.TraceContext{TraceID,ParentID,Flags string}`와
`domain/trace.ParseTraceparent(string)(TraceContext,bool)`을 추가한다.
v00 길이/hex/nonzero ID/flags를 검증한다. CLI는 환경을 한 번, HTTP는 요청 header만
읽어 context에 넣는다. 다른 요청이나 서버 환경으로 fallback하지 않는다.
Claude 자동 전파는 telemetry/beta exporter가 활성화된 경우만 기술하고,
custom base URL은 `CLAUDE_CODE_PROPAGATE_TRACEPARENT=1`도 필요하다.

**Acceptance:** 큰 중간 행·손상 행 뒤 sentinel, 정상 JSON/JSONL을 CLI로 분석해
complete와 warnings를 확인한다. 세 host export fixture를 CLI로 읽어 duplicate,
unknown/0, reset, redaction을 확인한다. traceparent 두 동시 요청이 섞이지 않아야 한다.

### I2: 측정과 재사용

**인계 상태:** parent가 duration 증분을 작업 트리에 반영했다. 변경 전 Go 전체
기준선과 네 probe의 deterministic RED는 parent가 확인했다. `execution-ledger.md`의
package GREEN `mon_6X3K5VHQTQR7JJEZ`는 exit 0이며 LSP 진단과 gofmt 출력도 없다.
`internal/adapter/verification/probe/contractauditworker/`의
`validation_duration_test.go`, `validation_contract_check.go`, `validation_tool_conformance.go`,
`validation_worker_lifecycle.go`, `validation_command_audit.go`는 명시적 인계 전까지
parent 독점 소유다. B는 이 다섯 파일을 되돌리거나 재설계하지 않는다.

별도 clock/helper를 추가하지 않는다. `verification/process.go:17-42`가 측정한
command duration을 보존한다. contract check/tool conformance/command audit는
AssertionStep에 `step.DurationMS`를 전달한다. worker lifecycle은 실제 실행한
enqueue/status/cancel/list의 DurationMS 합계를 사용한다. setup/parsing 시간이 아닌
command 실행 시간임을 명시한다. end-to-end wall-clock 측정이라는 주장은 하지 않는다.
조기 child 실패는 기존 반환을 보존하고 assertion 실패에도 측정한 command 시간은 버리지 않는다.

**미완료 후속 증분:** 현재 duration 수정에는 DTO/schema 변경이 없다. B가
`StepResult.Reused bool`,
`SummaryStep.Reused bool`, duration stat `ReusedCount int`를 추가한다.
reused는 pass/coverage에는 포함하지만 timed slowest와 count/p95 표본에서는 제외한다.
reused-only label은 count=0,reused_count>0이다.
selfverify contract는 **v7**, summary envelope는 **v1 유지**다.
옛 v6 결과는 읽되 v7 baseline 재사용은 contract hash 불일치로 거부한다.
옛 통계의 reused 여부를 추정하지 않고 v6와 v7 duration series를 합치지 않는다.

**Acceptance:** 기존 `TestSuccessfulProbesPreserveCommandDurations`의
137/149/163ms 보존과 worker 11+13+17+19=60ms를 유지한다. clock·sleep은 필요 없다.
측정 1개+reuse 1개와 reuse-only fixture, 실패 child를 검사한다.
실제 self-verify summary에서 count/reused_count와 v7을 확인한다.

### I3 / I4: 진단과 구조화 결과

`IntegrationStatus.Hosts []HostIntegration`을 추가한다.
`HostIntegration{Host,ConfigPath,Transport,SkillPath string; Installed,Linked,Configured,Discovered,Connected,Protocol Observation}`;
`Observation{Status,Source,ObservedAt,Reason,HostVersion,RequestedRevision,NegotiatedRevision string; Features []string}`.
status는 verified/failed/unknown/not_checked다. 기존 JSON 필드는 유지한다.
기본 inspect는 파일/파싱만 수행하며 host receipt가 없으면 뒤 세 관측은 not_checked다.
실제 host catalog와 docs_index 응답만 발견·연결 증거다.
`inspect --host-receipts FILE`은 schema=1의
`{schema_version,hosts:[]HostIntegration}`을 읽는다. host version/config SHA가 현재와
다르면 stale로 unknown 처리하므로 Observation에 `ConfigSHA256 string`도 둔다.
MCP harness_inspect에도 `host_receipts?:string`을 추가한다.
receipt의 source는 실제 host run artifact 경로여야 하며 synthetic source는 verified로
승격하지 않는다. ConfigSHA256는 secret을 제거한 issueops entry의 canonical JSON hash다.
(2026-10-03 Z 보완, D-K2) 처음에는 parsed config 전체를 해싱하기로 했다. 그러나 `~/.claude.json`의
`numStartups`처럼 host가 매 실행 바꾸는 필드 때문에 receipt가 곧바로 stale이 되므로,
issueops entry만 해싱한다(`internal/adapter/inspect/hostconfig.go:152-157`). 같은 형태의 entry를 쓰는
Claude와 Omo는 같은 hash를 가질 수 있다. receipt는 config_path도 현재 경로와 같아야 하고,
artifact 내용에 `docs_index`(발견·연결) 또는 주장한 revision(protocol)이 있어야 verified를
유지한다(D-K1, `internal/adapter/inspect/receipts.go:196,205,213`).
`inspectcontract.Options{CodexHome,HostReceipts string}`을 추가하고
`Observer.Inspect(root,target,home,version,skillName string, options inspectcontract.Options) InspectInfo`
로 확정한다. K가 새 입력을 연결하고 Z가 나머지 호출자를 직렬 갱신한다.

`mcp.Tool.OutputSchema map[string]any`, `Annotations *ToolAnnotations`를 추가한다.
`ToolAnnotations{ReadOnlyHint,DestructiveHint,IdempotentHint,OpenWorldHint *bool}`다.
ToolMaps/SDK 등록까지 전달한다. harness_inspect/docs_index의 실제 DTO를
JSON Schema 2020-12로 모델링하고 nil array는 null도 허용한다.
기존 JSON text와 같은 object를 structuredContent로 제공하고 결과 schema를 검사한다.
두 도구만 readOnly=true/openWorld=false; mixed mutation 도구는 read-only로 표시하지 않는다.
direct Markdown과 tool isError/protocol error 구분은 보존한다.
invalid output은 원문 없이 protocol internal error -32603으로 반환한다.

**Acceptance:** 임시 HOME, CODEX_HOME, broken symlink, malformed/duplicate config,
receipt 없음/stale를 검사한다. SDK round-trip에서 schema/annotation/text parity,
null slice, invalid output, exact Markdown을 검사한다. 세 실제 host가 docs_index를
호출한 receipt로만 connected를 verified로 바꾼다.

### I6: 요청 내 Git 관측

preflight는 `git log -10 --format=%h%x00%s%x00%B%x00` 한 번으로 네 history projection을
만든다. NUL 3-field record를 파싱하고 record 사이 newline만 제거한다.
정상 Git commit message의 NUL 금지를 전제로 하며 malformed tuple은 기존 빈/error
의미로 처리한다. display trim과 limit 1/5/10을 보존한다.
runner seam은 `func(string,...string)(int,string,string)`이다.

Next의 branch/worktree projection은 하나의 즉시 관측 결과를 공유한다.
root literal+argv가 다르면 공유하지 않는다. memo는 요청 내 **관측 단계**까지만
살고 fetch/외부 작업/새 관측 단계에서 폐기한다. HEAD/status/권한 검사를 장기간
cache하지 않는다. readiness의 기존 제한된 branch/status memo도 같은 경계를 따른다.
동시 Git 작업에 대한 atomic snapshot 보장은 추가하지 않는다.

**Acceptance:** history 호출 4→1, 같은 단계 branch 중복 2→1을 recording runner로
검사한다. 다른 root/ref, fetch, 두 번째 요청, barrier 뒤 외부 변경은 fresh여야 한다.
임시 실제 Git repo에서 preflight projection과 다음 요청의 변경 관측을 확인한다.

### I7: metadata 발견

1024 raw cap 제안은 버린다. `ReadDir(128)`을 EOF까지 읽으면서 우선순위
필수→선택→기타 사전순의 상위 64개만 bounded heap에 보존한다.
메모리는 batch+64개, body read는 64×8192 bytes 이하이며 raw 이름 scan은 O(n)이다.
directory read 오류면 scan_truncated=true, 부분 결과임을 표시한다.
선택 후 기존 open-root/Lstat/fstat/symlink/256KiB oversize 검사를 유지한다.
8KiB 안에서 닫히지 않은 frontmatter는 canonical fallback, 밖의 H1은 빈 title이다.
잘린 마지막 줄은 버려 부분 UTF-8/metadata를 해석하지 않는다.

`domain/projectdoc.CatalogOmissions{Oversize,Unreadable,OverCap,HeaderTruncated int; ScanTruncated bool}`,
`CatalogStats{FilesOpened,BytesRead,DirBatches int}`를 추가한다.
`DiscoverProjectDocsReport(string)([]ProjectDocCatalogEntry,CatalogOmissions,CatalogStats)`
를 새 표면으로 하고 기존 Discover 함수는 entries wrapper로 유지한다.
CatalogService.DiscoverReport는 같은 함수 타입,
ProjectDocCatalogContext.Omitted는 `*CatalogOmissions`다.
생략 없는 compact 출력은 동일하고 있으면 정확한 count와 truncation 상태를 덧붙인다.

**Acceptance:** 생성 순서가 다른 200개/1200개에서 같은 선택 집합,
필수 문서 생존, 9×240KiB의 header byte 상한, omission count, symlink 거부를 확인한다.
실제 hook catalog JSON/compact 출력도 검사한다.

### I8: 단계 지연

SpanObservation에 `Acquired bool; Callback,Commit *time.Duration; Total time.Duration;
CommitCount int; CommitCoverage string`을 추가한다.
wait=local+SQLite 대기, callback=inclusive fn, commit=data Commit 구간 합,
hold=lock 획득부터 rollback, total=gate 반환까지다. 서로 더하지 않는다.
commit은 callback의 부분집합이며 observer 시간은 제외한다.
context-local accumulator를 쓰고 nested/different-root/concurrent 요청을 분리한다.
Store의 Put/Delete 세 호출은 각각 단일 mutation Apply로 바꿔 기존 가시성 경계를
유지한다. 미관측 autocommit은 null/unknown, 관측된 미실행만 0이다.
panic은 error outcome을 기록한 뒤 재전파한다. 기존 slow/error filter를 유지한다.

**Acceptance:** fake clock 단계값, channel 기반 contention/cancellation,
observer reentry, commit failure/미실행/unknown을 검사한다.
실제 임시 SQLite의 actionable JSON에 단계값을 확인하고 nil/no-op/JSON observer
benchmark의 ns/op·allocs/op를 기록한다. 임의 성능 합격률은 만들지 않는다.

## 5. 단일 쓰기 소유권과 phase DAG

각 owner는 코드와 인접 테스트를 함께 소유한다. 조사 노트의 파일 목록은 이 규칙보다
우선하지 않는다. 아래 나열하지 않은 공유 파일은 **Z만** 수정한다.
기존 파일의 정확한 범위는 각 입력 노트의 Ownership 목록이며 아래 표의 직렬 이관이
그 목록을 덮어쓴다. 신규 파일은 표의 패키지 owner만 만든다.

| Owner | 독점 범위 | 선행 |
|---|---|---|
| A | I1 trace decode/domain analysis/application service/contract types 및 테스트 | 없음 |
| B | I2 reuse/sample-count, selfverify steps/summary, selfaugment stats/contract version; duration 다섯 파일은 parent 인계 후만 | parent duration GREEN·소유권 인계 |
| C | I6 preflight, readiness_git, issueops_next_wiring 및 테스트 | 없음 |
| D | I7 projectdoc catalog, hookprompt DTO/service/renderer, hook_facade | 없음 |
| E | I8 sqlstore 전체 변경, issueopsrecord store/observer 및 테스트 | 없음 |
| F | I5 go.mod/go.sum, revision compatibility 테스트 | 없음 |
| G | authority domain/application/outbound, CLI authorize, 기존 lease/completion/cycle/reconcile/replacement/publication/sync-base/preparation/branch/delegation의 actor guard, §2에 열거한 모든 호출자·9개 wiring·인접 테스트 | E |
| H | I10 mcpcli transport/dispatch/execution deps, service 명령, HTTP composition | F,G |
| J | I10 codex/claude/omo installer/activation/templates, adapter/mcpservice, service units, update/bootstrap | H |
| K | I3 inspect DTO/adapter, basiccli inspect 입력, app.go inspector wiring | J |
| L | I4 MCP catalog types/schemas/assembly, mcp_sdk_server 및 테스트 | H,K |
| M | I9 trace/usage/basiccli trace 입력, traceparent, issueopsrecord observer correlation | A,E,H |
| Z | authority contract/port, model.VerifiedActor와 issueopsauthorization contract alias, 공용 CLI/MCP catalog, port/install 및 mcpservice DTO, 남은 composition, golden, architecture inventory, 운영 문서 | 아래 순서 |

Z는 먼저 G/H/J가 사용할 catalog·install DTO를 이 계약대로 추가한다.
E는 G와 협의 없이도 여기 정한 WithRecordGuard와 Apply 경계를 구현한다.
H가 `mcp_sdk_server.go`를 넘긴 뒤에만 L이 수정한다.
Z-foundation의 MCP schema 추가가 끝난 뒤에만 L이 그 파일을 수정한다.
G의 native actor caller wiring 완료 뒤에만 H가 같은 composition 파일을 수정한다.
A가 trace 공유 파일을 넘긴 뒤에만 M이 수정한다.
E가 `issueopsrecord/observer.go`를 넘긴 뒤에만 M이 수정한다.
Z의 후속 composition 수정은 C/D/H/K/M 완료 뒤에만 한다.
H는 service CLI에 주입된 port만 호출하며 J의 구현을 기다리는 의존성 cycle을 만들지 않는다.
첫 구현 단계는 **(A,B,C,D,F) → 독립 검증**으로 병렬 실행한다.
메인은 이들과 쓰기 범위가 겹치지 않는 Z-foundation을 준비한다.
A/B/C/D/F는 authority DTO를 소비하지 않으므로 Z 완료를 기다릴 이유가 없다.
후속 경로는 **Z-foundation → E → G → H → (J,M) → K → L → Z-integration**이며,
H는 F의 검증 완료도 요구한다.
B에는 별도로 **parent duration GREEN·소유권 인계 → B** 의존성이 있다.
각 화살표는 변경 파일과 focused 증거를 확인한 뒤에만 열린다.
Z는 마지막에 golden/architecture inventory를 한 번 갱신하고 승인된 shared 방향으로
in-process-only ADR 및 runtime/install/usage 문서를 수정한다. generated openwiki는 제외한다.

## 6. 통합 acceptance와 인계

**Acceptance criteria:** 먼저 각 owner가 기존 테스트를 읽고 RED→GREEN을 남긴다.
시간은 fake clock, race는 subscribe-before-trigger channel과 bounded context timeout으로
검증한다. sleep/polling·prose pinning·기존 guard 약화는 금지한다.

권한 테스트는 잘못된 key/token/schema/expiry/root/live receipt,
발급 ancestry 밖 actor, actor 인자 혼합, missing bearer, foreign Origin,
재발급과 대기 mutation, release/complete race, commit 전 취소를 포함한다.
두 caller의 동일 repo grant가 서로의 active lease 권한으로 승격되지 않아야 한다.
같은 OS 사용자가 다른 credential 파일을 읽으면 방어된다는 거짓 테스트는 만들지 않는다.
native authorize 한 번 뒤 HTTP prepare→claim→release→resume→complete를 실행한다.
별도 fixture로 HTTP takeover/reseed/reconcile을 확인한다. generation 전환 후 같은
grant로 정상 요청은 성공하고 stale generation은 기존 core에서 거부되어야 한다.
HTTP VerifiedActor의 ancestry=nil, native ancestry 위조 거부, 서버/host restart 차이도 확인한다.
각 workspace family는 A/B repo 동시 요청, root 누락/불일치, 상대 경로, ID의 foreign repo를
검사한다. shared 서버를 C 디렉터리에서 실행해도 C에는 읽기·쓰기·명령이 없어야 한다.

SDK matrix는 stdio/HTTP × 2025-11-25/2026-07-28, unsupported revision,
metadata/header mismatch, JSON/SSE, 2026 subscription/disconnect를 포함한다.
서비스 두 번 start, 포트 충돌, stale PID, drain, update build_id, dry-run 무쓰기도 검사한다.

실제 표면은 격리 HOME/state에서 세 host 각각 stdio와 직접 HTTP로
tools/list와 docs_index를 실행하고, 각 host에서 짧은 native authorize 뒤 HTTP
prepare/claim 및 holder mutation을 성공시켜 상태를 readback한다. host/version/revision/transport,
redacted 결과, server PID를 증거로 남긴다. 세 host가 같은 PID를 사용하고
추가 stdio proxy가 없어야 한다. 설정 readback만으로 성공 처리하지 않는다.

Z의 최종 구현 검증: `go vet ./...`, `go test ./... -count=1`,
`go test -race ./... -count=1`, `go build -o bin/issueops ./cmd/issueops`,
두 contract golden suite, API static-check와 기록된 api-doc review, 위 real-surface
시나리오다. Python 기준선은 `execution-ledger.md`에 기록된 parent run
`mon_G5X30VS3R28RVHTF`의 exit 0을 사용한다. 격리 환경
`/tmp/issueops-ten-improvements-01a0fa31/venv`의 Python 3.12.14와
system-site-packages=false를 직접 확인했다. 변경 후에는 해당 venv의 bin을 PATH에
앞세워 `python3 scripts/python_suite_runner.py`를 실행한다. 전역 설치/새 skip은 금지한다.
산출물 경로는 repo-relative 또는 `$HOME`로 쓰고 로컬 사용자 이름은 넣지 않는다.

현재 설계 산출물 검증은 이 파일의 coverage/인터페이스/소유권 수동 점검과
`git diff --check -- .issueops/plans/agent-improvements-2026-10-02/contract.md`뿐이다.
구현·설치·commit/push는 이 설계 작업의 인계 경계 밖이다.
