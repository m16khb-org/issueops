# opencode v2를 다섯 번째 first-party host로 통합

## TL;DR
- Summary: opencode 2.0.32를 `codex, claude, omo, omp` 뒤의 다섯 번째 first-party host로 추가한다. omp가 받는 모든 통합 면(설치·활성화·검증, catalog 주입, actor·receipt, role model, 실행 profile, trace, host-probe, MCP enum·golden·문서)을 opencode 전용 adapter로 구현하고, lease를 쥐는 native receipt는 `--standalone` 세션의 전용 `serve --stdio` 프로세스로 제한한다.
- Deliverables: `internal/adapter/opencode` 설치 adapter, `hostprotocol` opencode plugin 생성기와 argv, `internal/domain/issueops`·`internal/application/authority`의 standalone receipt 규칙, agentmodel opencode 절, trace `opencode-json`, hostprobe opencode runner, 갱신된 golden·템플릿·ADR·문서·스킬.
- Effort: XL
- Parallel: NO — 한 owner가 순서대로 구현한다(host enum이 모든 golden에 걸려 병렬 편집 시 충돌한다).
- Critical Path: T1 → T2 → T3 → T8 → T9 → T10

## Context

### Original Request
"opencode를 omp처럼 issueops에 완전통합 하고 싶어". 사용자 선택: IssueOps 사이클로 진행, lease 변경은 `--standalone` 세션만 허용.

- lifecycle ID: `io-7426b49dc042`
- issue: https://github.com/m16khb-org/issueops/issues/568
- branch: `568-opencode-first-party-host`, base `main` @ `e65a62185ba55e54c8e59c8fd0bab7b84d84d358`
- 승인 범위·종료점: 전체 사이클(구현 → 정리 → 문서 → 검증 → 커밋·푸시 → draft PR 발행 → execution complete). merge와 cleanup은 별도 승인.
- 세션 인계: `execution prepare --confirm` 뒤 환경별 자동 세션 인계를 적용한다. 준비 세션은 Herdr 안의 omp 세션이므로 Herdr 서버·호환성·omp 실행 가능 여부를 확인하고, 쓸 수 있으면 같은 worktree의 새 Herdr omp 세션으로 인계한다. 아니면 현재 세션에서 이어간다. 자동 인계는 승인 범위를 넓히지 않는다.

### Interview Summary
- 진행 방식: IssueOps 사이클(사용자 답변).
- lease 소유: `--standalone` 세션만 허용(사용자 답변). 공유 서비스 세션은 fail closed.
- opencode v2 사실은 설치된 2.0.32 바이너리 문자열, `--help`, `opencode api`, 임시 HOME/XDG의 mock provider 실험으로 확인했다(세부는 아래 Gap Analysis와 각 TODO References).

### Gap Analysis
- **공유 서비스와 receipt**: `--standalone`이면 셸 → `…/bin/opencode serve --stdio --port 0`(comm 절대 경로) → `opencode` TUI 또는 `opencode run` 순서이고, `serve --stdio`는 클라이언트가 SIGKILL로 죽어도 함께 끝난다(실측). 공유 서비스(`opencode serve --service`)는 처음 접속한 클라이언트가 detached로 띄우므로, 그 클라이언트(일반 TUI)가 살아 있는 동안에는 셸 → `serve --service` → `opencode` TUI 사슬이 standalone과 basename만으로는 구별되지 않는다(리뷰 1라운드 지적, 바이너리 문자열 `detached:!0`와 `["opencode","serve","--service"]`로 확인). 그래서 receipt 규칙은 프로세스 argv로 판정한다: receipt 실행 파일 basename이 `opencode`이면 그 프로세스 argv의 첫 opencode 토큰 바로 다음이 `serve`이고 그 뒤에 `--stdio`가 있어야 한다(프롬프트 문자열로 참이 되지 않도록 위치를 고정, T1). 선언 host가 `opencode`면 receipt basename도 `opencode`여야 한다(셸·zsh 같은 오래 사는 조상을 receipt로 쓰는 우회를 막는다). receipt basename이 `opencode`인데 선언 host가 다른 경우도 같은 argv 규칙을 적용한다(명시적 `--host codex … --session-executable …/opencode` 우회 차단).
- **검증 위치**: `NormalizeNativeActor`(`internal/domain/issueops/native_actor.go:24-42`)의 호출자는 `internal/application/authority/service.go:211` `verifyNative` 하나이고, claim·prepare(`service.go:139`)·mutation authority(`internal/application/issueopscycle/mutation_authority.go:55-67`)·`mcp authorize`(`service.go:59`)가 모두 이 verifier를 지난다. 명시적 ACTOR_FLAGS 경로(`executioncmd/execution.go:200-213`)는 CLI의 `reusableNativeProcessAncestry`를 거치지 않는다. 따라서 host↔receipt basename 결합은 순수 규칙으로 `NormalizeNativeActor`에, argv 판정은 관측이 필요하므로 `verifyNative`에 둔다. argv는 `authorityport.ProcessInspector`를 구현한 adapter가 선택 interface `ProcessArgumentsInspector`(`Arguments(ctx, receipt) ([]string, error)`)로 제공하고, `ps -ww -o lstart=,args= -p PID`의 시작 시각이 receipt와 같을 때만 argv를 돌려준다(PID 재사용 방지). 이 interface가 없거나 관측이 실패하면 opencode receipt는 거부한다. argv는 판정에만 쓰고 record에 저장하지 않으므로 record schema는 바뀌지 않는다.
- **공유 서비스 세션에 남는 기능**(코드로 확인, `cmd/issueops/mcpcli/mcp_tool_authority.go:62-117`, `mcp_request_dispatch.go:33-45`): 공유 서비스 셸에서는 `mcp authorize`가 거부된다. HTTP(darwin/linux 기본)에서 `authority_file`이 필요한 workspace 도구(`project_docs_*`, `gates_*`, `loop_*`, `command_policy_*`, `issueops_execution`, `repo`·`workspace_root`를 준 `harness_inspect` 등)는 `authority_required`로 실패하고, 서버 범위 도구(`docs_index`, `state_*`, `skill_manifest`, `commit_policy`, `channel_*`, `self_*`)와 인자 없는 `harness_inspect`는 동작한다. stdio에서는 workspace 도구가 capability 없이 stdio 서버의 native ancestry로 동작하지만, stdio MCP 프로세스에는 `OPENCODE_SESSION_ID`가 없어 `issueops_execution` 변경은 actor를 얻지 못해 거부된다. 스킬과 catalog는 두 경우 모두 동작한다. 이슈 본문의 "MCP 읽기 도구는 동작" 문장은 이 구분과 다르므로 인계 전에 `feedback add --classification contract_change`로 기록하고 이슈를 갱신한다.
- **세션 ID**: opencode는 셸 도구 환경에 `OPENCODE_SESSION_ID`를 직접 넣는다(실측: run의 sessionID와 같음). omp처럼 plugin이 `process.env`에 쓰면 공유 서비스의 다른 세션으로 새므로 쓰지 않는다. 서브에이전트 셸의 값은 자식 세션 ID라 holder와 달라 lease 변경이 거부된다(의도된 fail closed).
- **catalog 주입**: v2에는 session start 이벤트가 없고 `session.compacted`도 없다(압축은 `session.compaction.started|ended|failed`). `ctx.session.hook("context", ev)`는 모든 agent-loop 모델 호출 전에 실행되고 `ev.system.push({type:"text", text})`가 압축 뒤 첫 호출에도 반영된다(실측). 따라서 plugin은 세션마다 한 번 catalog를 읽어 캐시하고 매 호출에 push한다. post-compact 재주입은 필요 없다. 이벤트에 `parentID`가 없으므로 자식 여부는 `ctx.session.get`으로 한 번 읽어 캐시한다.
- **스킬**: `~/.config/opencode/skills`와 `~/.claude/skills`에 같은 이름이 있으면 한 번만 나열되고 config 쪽이 이긴다(실측). 다른 host와 같게 `~/.config/opencode/skills`에 symlink를 만든다.
- **설정 파일**: 같은 디렉터리에 `opencode.json`과 `opencode.jsonc`가 모두 있으면 둘 다 읽고 `.jsonc`가 이기며, 같은 `mcp.servers.x`는 객체째 교체된다(실측). issueops는 `opencode.json`만 관리하고 `.jsonc`는 주석 보존을 위해 고치지 않는다. `.jsonc`는 문자열·escape를 인식해 `//`·`/* */` 주석과 trailing comma를 제거한 뒤 읽는다(저장소에 JSONC 파서가 없어 opencode adapter 안의 작은 함수로 둔다). 읽은 결과가 `mcp.servers.issueops`를 가지면 issueops 항목을 가리므로 opencode 설치를 오류로 멈춘다. 읽기·파싱에 실패하면 설치를 막지 않고 "opencode.jsonc could not be read; make sure it does not define mcp.servers.issueops" 경고 메시지만 남긴다(다른 host의 설치를 막지 않기 위해).
- **역할 모델**: TUI에는 `--model`이 없다. `--standalone`에서 `OPENCODE_CONFIG_CONTENT={"model":"P/M"}`는 기억된 모델(`model.json` recent)보다 우선한다(실측). root `model`의 `#variant`는 버려지므로 대화형 실행에는 effort를 넣지 않는다. `opencode run --model P/M#E`는 variant를 받고, 모델에 없는 variant는 exit 1 `Variant unavailable`로 실패한다(실측). variant 이름은 모델마다 달라 issueops의 docs-only(`medium`)·escalation(한 단계 상승) 조정이 존재하지 않는 variant를 만들 수 있다.
- **print argv**: `opencode run`은 positional 프롬프트에 공백이 있으면 따옴표가 섞여 들어가므로(실측) 프롬프트는 표준 입력으로 넘긴다. `--auto` 없이도 기본 build agent 권한으로 멈추지 않는다(실측).
- **trace**: `run --format json`의 `step_finish` 줄이 `tokens{input,output,reasoning,cache{read,write}}`를 가진다. 마지막 step은 `step_finish`를 내지 않는 경우가 있어(실측) 합계가 실제보다 작을 수 있다. collector는 관측한 줄만 합산하고 이 한계를 ADR·운영 문서에 적는다.
- **MCP stdio**: stdio MCP 프로세스는 서버(공유 서비스 또는 standalone serve)·location당 하나이고 서버 env와 항목의 `environment`만 받으며 `OPENCODE_SESSION_ID`를 받지 않는다(실측). 위 "남는 기능" 절의 결과가 그대로 적용된다.
- **cmux**: cmux가 이 머신에 없다(`which cmux` 실패). launcher가 절대 경로로 exec하므로 receiver comm이 절대 경로가 되어 `sameExecutableIdentity`(`internal/adapter/cmux/artifact.go:201-211`)를 통과할 가능성이 높지만 실측하지 못했다. argv는 unit test로 고정하고, 실측하지 못한 사실을 ADR에 적는다.
- **live 실행 env**: 구현 세션은 Herdr 안의 omp 세션이라 셸에 `ISSUEOPS_OMP_SESSION_ID`가 있고, `--standalone`의 셸 도구는 클라이언트 env를 물려받는다(실측). 그대로 두면 `OPENCODE_SESSION_ID`와 함께 ambiguous가 된다. opencode 바이너리는 Cellar 설치를 감지해 `brew upgrade`를 실행하는 updater를 갖고 있고 `OPENCODE_DISABLE_AUTOUPDATE=1`로만 건너뛴다(probe가 이 env를 넣고 실험함). 그래서 live 테스트(T9)와 probe runner(T7)는 자식 env를 허용 목록으로 새로 만든다. 목록은 hostprobe의 기존 `isolatedHostEnv`(`internal/adapter/hostprobe/runner.go:533-564`: `PATH`·`HOME`·`USER`·`TMPDIR`·`LANG`·`LC_*`)에 `TERM`을 더한 것이고, `HOME`·`XDG_*`·`ISSUEOPS_STATE_DIR`은 임시 경로로 덮는다(T7은 `isolatedHostEnv(deps, "TERM")`과 `replaceEnvironmentValue`를 그대로 쓰고, 다른 package인 T9 테스트는 같은 목록을 테스트 안에 둔다). 여기에 `OPENCODE_DISABLE_AUTOUPDATE=1`, `OPENCODE_DISABLE_MODELS_FETCH=1`을 넣고, 다른 host의 세션 ID와 테스트가 정하지 않은 `OPENCODE_*`는 넣지 않는다. mock이 시키는 명령은 임시 issueops 바이너리를 절대 경로로 부른다.

## Work Objectives

### Core Objective
opencode 세션이 omp 세션과 같은 issueops 기능(스킬, MCP, catalog, actor, lease, role model, 인계, trace, probe)을 받고, lease 소유는 `--standalone` 세션으로 제한된다.

### Deliverables
- `internal/adapter/opencode/`: `install.go`, `mcp.go`, `activation.go`, `dependencies.go`와 테스트.
- `internal/adapter/hostprotocol/opencode.go`: lifecycle plugin 생성기, 테스트.
- `configs/opencode/opencode.json`, `configs/opencode/issueops.js` 추적 템플릿.
- host 목록·검증기·usage·golden·inventory 갱신.
- ADR `adr/2026-10-18-opencode-first-party-host.md`와 현행 문서·스킬 갱신.

### Definition of Done (verifiable conditions with commands)
- 계획의 G1–G11 게이트가 모두 met(`issueops gates check --file .issueops/issues/568/gates.md --write --json` exit 0).
- `gofmt -l $(git ls-files '*.go')` 빈 출력, `go vet ./...`, `go test -race ./... -count=1`, `go build -o bin/issueops ./cmd/issueops` 성공(G9).
- CI와 같은 격리 실행(임시 clone + 임시 HOME에 stdio 설치 + self-verify)의 `ok`와 `termination_eligible`이 true(G11). 사용자 실제 HOME에서 실행한 self-verify는 머지 뒤 source checkout에서 `issueops update`를 하기 전까지 opencode native integration 검사에서 실패하는 것이 정상이다(설치 전 상태). worktree에서 실제 설치를 하지 않는 이유는 `.issueops/cautions/integrations.md` §20 마지막 항목과, linked worktree 설치가 source checkout을 stable root로 쓰는 동작(`internal/adapter/install/native_root.go:38-66`) 때문이다.

### Must Have
- receipt 규칙이 core에서 강제된다: host↔receipt 결합은 `NormalizeNativeActor`, `serve --stdio` argv 판정은 `verifyNative`(모든 claim·prepare·mutation·authorize가 지나는 유일한 verifier).
- 사용자 opencode 설정의 다른 키·다른 plugin 파일은 그대로다.
- plugin 생성기 출력이 MCP·lifecycle SSoT이며 템플릿과 바이트 단위로 같다.

### Must NOT Have
- opencode 설치·업그레이드·서비스 재시작을 대신하는 코드.
- `process.env`에 세션 ID를 쓰는 plugin.
- `opencode.jsonc` 편집.
- 공유 서비스 receipt 허용, 서브에이전트 셸 lease 변경 허용.
- worktree에서 실제 `issueops install`/`update` 실행(사용자 홈과 source `bin/issueops`를 바꾼다; `.issueops/cautions/integrations.md` §20 마지막 항목).

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 제2장 안전 불변식: "핵심 정책은 host adapter가 아니라 Go core에 둔다." → receipt 규칙은 `internal/domain/issueops`(`NormalizeNativeActor`와 순수 판정 함수)와 `internal/application/authority` `verifyNative`(argv 관측 판정)에 두고 plugin에는 catalog 전달만 둔다.
- `.issueops/CONSTITUTION.md` 제2장 문제 해결 원칙: "에이전트, hook, relay, retry, monitor, orchestration의 모든 반복은 유한한 상한과 명시적인 성공·실패·취소·timeout·no-op 종료 경로를 가져야 한다." → plugin의 프로세스 호출은 timeout을 두고, 세션 캐시는 상한(512)을 둔다. 이벤트 구독 루프는 만들지 않는다.
- `.issueops/ARCHITECTURE.md` 의존 방향 불변식: "새 host 추가 시 공통 domain/application 정책을 복제하지 않고 `port.HostInstaller` 구현체와 composition-root wiring만 추가한다." → 설치는 `port.HostInstaller` 구현(`internal/adapter/opencode`)과 `cmd/issueops/issueopsapp` wiring으로만 붙인다. domain 변경은 host 목록과 opencode receipt 규칙에 한정한다.
- `.issueops/architecture/host-integration.md` 체크리스트: "host adapter 변경: core contract를 복제하지 않았는지 확인하고 `internal/adapter` contract matrix golden으로 Codex/Claude/Omo/omp 설치 표면이 drift되지 않았는지 검증한다." → `native_install_contract_matrix.golden.json`을 opencode 포함으로 재생성하고 다른 host 항목의 diff가 순서·개수 외에 없음을 확인한다.
- `.issueops/conventions/go-and-packages.md:61` 표: "`adapter/hostprotocol` | host별 hook JSON, 실행 인자, Omo/omp extension 코드 생성" → opencode plugin 생성기와 argv는 `hostprotocol`에 두고 root에서 installer·verifier·probe에 주입한다(같은 파일 :65).
- `.issueops/conventions/state-policy-and-hooks.md:44`: "`issueops hook`의 subcommand는 `session-start`, `subagent-start`, `post-compact` 셋이다." → 새 hook subcommand를 만들지 않고 plugin이 `session-start --json`(루트)과 `subagent-start --json`(자식, stdin `agent_type`)을 호출한다.
- `.issueops/CAUTIONS.md:43-46`: "Omo·omp MCP catalog는 server config hash로 장기 cache되므로 installer가 advertised catalog SHA를 config env에 포함해 binary-only schema update도 fresh session에서 재조회되게 한다" → opencode MCP 항목에도 같은 digest(HTTP는 `X-Issueops-Mcp-Catalog-Sha256` 헤더, stdio는 `ISSUEOPS_MCP_CATALOG_SHA256` environment)를 넣는다. opencode의 디스크 캐시는 확인되지 않았지만 설정 객체가 바뀌어야 reload가 재연결하므로 같은 규칙을 따른다.
- `.issueops/cautions/integrations.md` §12: "**외부 CLI를 통한 관찰은 쓰기다.** dry-run·preview·readiness 경로는 외부 프로세스를 spawn하지 않는다(`exec.LookPath` 수준의 존재 확인만 허용)." → install dry-run·native integration 검증·inspect는 `opencode`를 실행하지 않는다. 버전 관측은 기존 `CommandHostVersion` 경로(inspect의 명시 호출)만 쓴다.
- `.issueops/cautions/integrations.md` §20 마지막 항목: "supervised IssueOps owner는 worker worktree에서 `scripts/install-native.sh` 또는 `issueops install`/`install-native`/`update`/`bootstrap`의 실제 실행으로 사용자 범위 통합을 교체할 수 없다." → 검증은 Go 테스트의 임시 HOME, `install --dry-run`, env-gated live 테스트로 한다.
- `.issueops/cautions/2026-08-28-verify-host-cli-flags-against-the-installed-version.md`: "Do not assume automation flags from another CLI version; inspect the installed host help and lock exact argv with a focused adapter test." → 모든 opencode argv는 2.0.32 `--help`로 확인한 플래그(`--standalone`, `--auto`, `--prompt`, `run --model --format`)만 쓰고 `argv_test.go`·`role_agents_test.go`로 고정한다.
- `.issueops/adr/2026-10-08-omp-first-party-host.md` Decision: "omp is the fourth first-party host, ordered after Omo everywhere (`codex, claude, omo, omp`; installers end with `agy`)." → opencode는 omp 뒤 다섯 번째, 설치기는 `codex, claude, omo, omp, opencode, agy`.
- `.issueops/adr/2026-10-08-omp-first-party-host.md` Decision: "A session id alone never authorizes a lease; Omo and omp ids together are ambiguous and fail closed." → `OPENCODE_SESSION_ID`도 후보 목록에 더해 다른 host ID와 함께 보이면 ambiguous로 거부한다.
- `.issueops/adr/2026-10-12-omp-role-models-resolve-from-the-agent-model-settings-files.md` Decision: "the resolver reads local then global layers before the built-in defaults for every host." → `opencode` 절을 Config에 더하고 같은 해석 순서를 적용한다. Consequences의 "older builds reject a file that contains an omo or omp section as an unknown field"도 opencode 절에 똑같이 적용되므로 ADR에 적는다.
- `.issueops/adr/2026-10-02-shared-streamable-http-mcp-and-caller-capability.md` Decision: "세 host의 user 설정에는 issueops entry 하나만 `url`+`Authorization` 헤더로 바꿔 넣고 0600으로 쓴다." → `opencode.json`은 bearer를 담으므로 0600으로 쓴다(기존 파일 mode가 더 넓어도 0600으로 좁힌다).
- `.issueops/adr/2026-10-15-remove-orca-integration-and-execution-mode-selection.md`: Orca launch profile·delivery가 없으므로 opencode에도 만들지 않는다.
- `.issueops/TESTING.md` 최소 완료 기준: "단일 self-verify의 완전한 성공 결과로 test/build/golden/docs/inspect 기본 게이트를 확인한다." → 최종 검증은 self-verify 한 번의 완전 성공으로 한다.
- `.issueops/testing/cli-mcp-and-hosts.md:29`: "Live 측정은 CI와 기본 self-verify에 포함하지 않는다. 기본 `contract conformance live` host 목록은 Codex/Claude로 유지한다." → opencode runner는 `--hosts opencode`와 명시 `--model opencode=…`일 때만 시작한다.
- `.issueops/CONVENTIONS.md:145`: "생성물은 직접 수정하지 않고 생성 스크립트와 source를 고친 뒤 재생성한다." → golden·inventory·configs 템플릿은 각 `-update*` 플래그와 installer 테스트로 재생성한다.
- `.issueops/ADR.md` Accepted baseline: "**First-party hosts:** Codex, Claude Code, Omo native, omp가 같은 shared skill, MCP, lifecycle activation contract를 사용한다." → opencode도 같은 계약을 쓰며 T10에서 이 문장과 근거 링크에 새 ADR을 더한다.
- `.issueops/cautions/audit-and-process.md` §10: "새 CLI/MCP/native skill 기능은 `issueops self-verify`의 테스트 또는 QA 단계에 smoke/fuzz evidence label로 승격한다." → native integration validator(self-verify battery가 실행)에 opencode 경로·MCP·plugin 바이트 검사를 더해 self-verify가 opencode 설치를 검증하게 한다(T3).
- `.issueops/cautions/audit-and-process.md` "긴 QA 파동" 절: "named test 출력의 `[no tests to run]`, 누락된 marker, 예상하지 않은 결합 token은 파동 실패다." → G1·G2·G5·G7·G10은 `--- PASS: <이름>`을 이름으로 요구한다.
- `.issueops/TECH_STACK.md` §3: "`github.com/dop251/goja` … (Omo extension JS를 실행하는 테스트 전용)." → 새 dependency 없이 goja로 opencode plugin을 테스트하고, 같은 줄의 용도 설명에 opencode plugin을 더한다(T10).
- `.issueops/OPERATIONS.md`와 route가 고른 모듈 `.issueops/operations/guides/hosts.md`: 대조했으나 이 변경을 제약하는 새 규칙 없음. hosts.md의 `## omp` 절 형식(경로 목록, 템플릿, lifecycle 설명, Checks)을 그대로 따라 T10에서 `## opencode` 절을 쓴다.
- `.issueops/adr/2026-08-12-omo-native-first-party-host.md` Decision: "Activation commits only after exact MCP semantics and lifecycle extension bytes are read back with filesystem identity evidence." → opencode activation readback도 MCP 의미와 plugin 바이트를 확인한 뒤 commit한다(T3).
- `.issueops/adr/2026-08-27-session-start-owns-compaction-context.md` 결과: "주입할 문서가 없으면 두 context hook 모두 `{}`를 낸다." → plugin은 `should_inject`가 false면 아무것도 push하지 않는다. post-compact를 쓰지 않는 근거(context hook이 매 호출 재적용)는 새 ADR에 적는다.

## 재사용하는 기존 구현

- `internal/adapter/installutil`: `MergeJSONMapFile`·`RemoveJSONMapEntry`·`VerifyJSONMapEntry`(`json_config.go:14-101`)를 parent 한 단계 대신 key 경로(`[]string`)를 받도록 일반화해 `mcp.servers.issueops`를 병합한다. 기존 호출자(extensionhost, agy, inspect 등)는 `[]string{"mcpServers"}`로 옮긴다. 새 병합 함수를 따로 만들지 않는다.
- `internal/adapter/extensionhost`: 설치 흐름(`install.go:43-95` 스킬 링크 → user MCP → lifecycle 파일 → 프로젝트 템플릿 → project-local)과 HTTP/stdio 항목·catalog digest 처리(`mcp.go`)를 opencode adapter의 구조로 따른다. 설정 파일 이름·key 경로·항목 모양이 달라 Spec 확장 대신 `internal/adapter/agy` 선례처럼 별도 package로 둔다(Spec에 파일명·key 경로·항목 변환 함수까지 넣으면 두 host만 쓰는 추상화가 커진다).
- `internal/adapter/hostprotocol/omo.go`·`omp.go`의 생성기 패턴(JSON contract 상수 + 짧은 JS)과 `omp_test.go`의 goja mock 실행 테스트를 opencode plugin 테스트에 그대로 쓴다.
- `cmd/issueops/issueopscli/executioncmd/execution.go:301-363`의 세션 env 후보 목록과 `nativeHostProcessExecutable`에 opencode를 더한다.
- `internal/domain/nativehost/identity.go`의 `ExecutableMatchesHost`에 opencode를 더한다(launcher 확인). receipt 판정 함수는 `internal/domain/issueops`에 둔다(foundation owner의 import 제약, T1).
- `internal/adapter/hostprobe/omp.go`의 runner 구조(LookPath, private env, preflight, episode 준비, argv, 결과 projection)를 opencode runner가 따른다. pi 전용인 auth snapshot·`OmoJSONL` projection은 재사용하지 않는다.
- `internal/adapter/trace/host_usage.go`의 collector dispatch에 opencode collector를 더한다. domain reducer(`internal/domain/trace/host_usage.go`)는 그대로 쓴다.

## 성능 영향

- hot path: opencode plugin의 `context` hook은 모델 호출마다 실행된다. 세션당 첫 호출에만 `issueops hook ... --json`을 한 번 실행하고(실측: plugin 안 `Bun.spawn` 1.4ms + issueops catalog 생성) 이후는 `Map` 조회와 배열 push 한 번(O(1))이다. 캐시는 512개 상한, 넘으면 가장 오래된 항목을 지운다. 자식 판별 `ctx.session.get`도 세션당 한 번이다.
- Go 쪽: host 목록이 4→5로 늘어 설치·검증·inspect의 상수 배 작업이 늘 뿐 복잡도는 같다. receipt 규칙은 ancestry 한 번 순회(O(n), n≤수십)로 기존 `slices.Contains`와 같은 차수다.
- 측정: T2 goja 테스트가 spawn 횟수(세션당 1회)를 고정하고, T9 live 테스트에서 두 번째 모델 호출에 spawn이 없음을 확인한다.

## 하위 호환성과 side effect

- **CLI/MCP 계약**: MCP `issueops_execution.host` enum에 `opencode`가 추가되어 advertised catalog digest가 바뀐다. `mcp_tools.golden.json`, `usage.golden.txt`, `response_contracts.golden.json`, `native_install_contract_matrix.golden.json`, DDD inventory를 재생성하고 `configs/{omo,omp,agy,opencode}` 템플릿의 digest를 갱신한다. 기존 host 값은 그대로 유효하다.
- **record schema**: 새 필드 없음. holder host 값으로 `opencode`가 저장될 수 있다. 이 값을 담은 record는 이전 바이너리에서 `native actor host must be …`로 거부된다(새 host 추가의 일반적 비호환, ADR에 적는다).
- **설정 파일**: `agent-models` 설정에 `opencode` 절을 쓰면 이전 바이너리가 unknown field로 거부한다(ADR 2026-10-12와 같은 성격, ADR에 적는다).
- **사용자 파일**: 설치는 `~/.config/opencode/opencode.json`(없으면 0600으로 생성, 있으면 `mcp.servers.issueops`만 교체하고 HTTP transport일 때 mode를 0600으로 좁힘), `~/.config/opencode/plugins/issueops.js`, `~/.config/opencode/skills/<skill>` symlink를 쓴다. `XDG_CONFIG_HOME`이 절대 경로면 `$XDG_CONFIG_HOME/opencode`를 쓴다(opencode의 `debug paths`와 같은 해석). 다른 키·plugin·`cli.json`·`tui.jsonc`·`opencode.jsonc`는 건드리지 않는다. `--project-local`은 `.opencode/opencode.json`만 다루며 HTTP면 `issueops_project` 항목을 지운다. `opencode.jsonc`가 `mcp.servers.issueops`를 정의하면 opencode 설치 오류로 전체 install이 실패한다(그 항목을 지우는 복구 절차를 운영 문서에 적는다).
- **기존 동작 변화**: opencode 외 host의 receipt·argv·설치 결과는 바뀌지 않는다. receipt 규칙은 opencode 실행 파일 receipt와 선언 host `opencode`에만 걸린다. `hostprotocol/argv_test.go:59`의 unknown host 예시는 `gemini`로 바꾼다. `authorityport.ProcessInspector`의 메서드는 그대로이고 선택 interface만 더하므로 기존 구현·테스트 fake는 바뀌지 않는다.
- **이슈 계약 변경**: 리뷰 1라운드에서 확인한 사실로 이슈 본문 일부가 달라진다. (1) 공유 서비스 세션에서 동작하는 MCP 범위(Gap "남는 기능"), (2) print argv는 `opencode run --standalone --model P/M#E`(`--auto`·`--format json` 없음, 프롬프트는 표준 입력), (3) standalone claim 성공은 live 테스트의 `mcp authorize`(claim과 같은 verifier)로, 공유 서비스 거부와 압축 뒤 catalog 유지는 단위 테스트와 2.0.32 실측으로 관측한다. 인계 전에 `feedback add --source plan-review --classification contract_change`로 기록하고 sync-issue로 본문을 고친 뒤 `feedback mark-issue-updated`를 기록한다.
- **롤백**: 커밋 revert로 돌아간다. 이미 설치된 사용자 opencode 파일은 이전 바이너리의 `install`이 관리하지 않으므로 수동 삭제 경로(`issueops.js`, `mcp.servers.issueops`, skills 링크)를 운영 문서에 적는다.
- **LLM 프롬프트**: 리뷰어·서브에이전트 프롬프트 본문은 바꾸지 않는다. catalog 텍스트는 기존 `FormatProjectDocCatalog` 출력을 그대로 쓴다.

## Verification Strategy
- Test decision: tests-after(각 TODO 안에서 구현과 테스트를 함께), framework는 Go `testing`과 goja.
- QA: 모든 TODO에 agent가 실행하는 시나리오. 사람이 조작하는 검증 없음.
- Evidence: 게이트 원장 `.issueops/issues/568/gates.md`의 EVIDENCE와 `.issueops/evidence/task-{N}-{slug}.txt`(ignored).

## Execution Strategy

### Parallel Execution Waves
- Wave 1: T1(host 식별·receipt)
- Wave 2: T2(hostprotocol), T4(agentmodel), T6(trace)
- Wave 3: T3(설치 adapter·검증·inspect), T5(인계 profile)
- Wave 4: T7(hostprobe runner)
- Wave 5: T8(enum·usage·golden 재생성), T9(live 테스트)
- Wave 6: T10(ADR·문서·스킬·이슈 동기화)
한 owner가 순서대로 실행한다. wave는 의존 순서를 보이기 위한 것이다.

### Dependency Matrix
| Task | Depends On | Blocks | Can Parallelize With |
|---|---|---|---|
| T1 | — | T2, T3, T4, T5, T7, T9 | — |
| T2 | T1 | T3, T5, T7, T9 | T4, T6 |
| T3 | T1, T2 | T8, T9 | T5 |
| T4 | T1 | T5, T8 | T2, T6 |
| T5 | T2, T4 | T8 | T3 |
| T6 | — | T8 | T2, T4 |
| T7 | T2, T3 | T8 | — |
| T8 | T3, T4, T5, T6, T7 | T9, T10 | — |
| T9 | T3, T8 | T10 | — |
| T10 | T8, T9 | — | — |

## TODOs

- [ ] 1. host 식별과 standalone receipt 규칙
  - What to do:
    - `internal/domain/nativehost/identity.go`: `ExecutableMatchesHost`에 `opencode`(basename `opencode`) 추가(launcher 실행 파일 확인용, 기존 host와 같은 패턴).
    - `internal/domain/issueops/native_actor.go`(새 `internal/` import 없음 — `internal/domain/issueops`는 foundation owner라 contract 외 `internal/` 패키지를 import하면 `TestProductionFoundationOwnershipHasNoForbiddenEdges`가 `domain_must_only_import_contract`로 실패한다, 4라운드 리뷰 실측): 새 순수 함수 `IsOpencodeExecutable(executable string) bool`(basename `opencode`, `.exe` 제거·소문자 비교, `path/filepath`·`strings`만 사용)과 `OpencodeStandaloneServer(fields []string) bool`: `ps args` 공백 분할 결과에서 **앞에서부터 처음 나오는** basename `opencode` 토큰을 실행 파일로 보고(그 앞의 조각은 공백이 든 경로의 일부), 그 바로 다음 토큰이 `serve`이고 그 뒤 토큰들 중에 `--stdio`가 있을 때만 true. 실측 argv는 `…/bin/opencode serve --stdio --port 0`(Round 4·5). 프롬프트 안의 `opencode serve --stdio` 문자열(예: TUI `opencode --prompt "opencode serve --stdio …"`, `opencode run "opencode serve --stdio"`)은 첫 opencode 토큰 다음이 `serve`가 아니므로 거부한다. 이 판정은 argv[0]의 basename이 `opencode`라는 가정에 기대며, 실행 파일 앞 경로 조각이 `opencode`로 끝나고 공백이 섞인 드문 설치는 fail closed로 거부된다(ADR에 기록). 같은 파일에서 `ValidateNativeActor`에 `opencode` 허용, 오류 문구를 "native actor host must be codex, claude, omo, omp, or opencode"로 바꾸고, `NormalizeNativeActor`에 순수 결합 규칙을 `ValidateNativeActor` 직후, ancestry 포함 검사(`slices.Contains`) 전에 둔다: 선언 host가 `opencode`인데 receipt가 `IsOpencodeExecutable`이 아니면 "opencode session process receipt must be the opencode standalone server" 오류(입력 receipt 값과 무관하게 결과가 정해진다).
    - `internal/port/authority`: 선택 interface `ProcessArgumentsInspector { Arguments(context.Context, model.NativeProcessReceipt) ([]string, error) }`. `internal/adapter/issueops/execution_lease_identity.go`의 `NativeProcessInspector`가 값 receiver로 구현하고 같은 파일에 `var _ authorityport.ProcessArgumentsInspector = NativeProcessInspector{}` 컴파일 단언을 둔다(production wiring `cmd/issueops/issueopsapp/authority_wiring.go:23`이 값 타입을 넘김). 구현: `ps -ww -o lstart=,args= -p PID`(기존 `nativeProcessProbeTimeout`, `LC_ALL=C`)를 읽어 시작 시각이 receipt와 다르면 오류, 같으면 `strings.Fields(args)`.
    - `internal/application/authority/service.go` `verifyNative`: `NormalizeNativeActor`와 `observeLive` 뒤, receipt가 `issueopsdomain.IsOpencodeExecutable`이면(선언 host와 무관) inspector를 `ProcessArgumentsInspector`로 단언해 argv를 읽고 `issueopsdomain.OpencodeStandaloneServer`가 false면 "opencode lease ownership requires a --standalone session (the session process is not `opencode serve --stdio`)" 오류. interface가 없거나 관측 실패도 같은 계열 오류로 거부. (application은 이미 `issueopsdomain`을 import한다, `service.go:14`.)
    - host 허용 목록과 오류 문구: `internal/domain/issueopslease/record_validation.go:67-69`, `internal/application/issueopspreparation/prepare.go:136-138`, `internal/adapter/issueops/execution_state.go:38-39`, `internal/domain/authority/authority.go:155-156`, `internal/domain/issueops/handoff_delivery.go:137-138`, `internal/domain/operationalhealth/classifier_identity.go:90-91`, `internal/contract/inspect/types.go:52-57`(`HostOpencode`).
    - `cmd/issueops/issueopscli/executioncmd/execution.go`: 후보에 `{Host:"opencode", SessionID: getenv("OPENCODE_SESSION_ID"), Source:"OPENCODE_SESSION_ID"}` 추가(omp 뒤). `nativeHostProcessExecutable("opencode", …)`는 basename `opencode`(같은 파일의 기존 host별 switch 패턴). `reusableNativeProcessAncestry`는 기존 규칙대로 가장 가까운 opencode 조상을 고른다(standalone이면 `serve --stdio`). 공유 서비스면 그 조상이 `serve --service`이고 core의 argv 규칙이 claim·authorize에서 거부한다.
    - 오류 문구를 고정한 테스트(`release_test.go:124`, `record_test.go:20`, `reseed_test.go:531`, `provisioner_edges_test.go:310`)와 host 루프 테스트(`execution_lease_boundaries_test.go:211`, `record_validation_matrix_test.go`), env 초기화 테스트(`execution_commands_test.go:318`, `runtime_instance_test.go:15,90`) 갱신. `hostprotocol/argv_test.go:59`의 unknown host를 `gemini`로.
    - 새 테스트(이름 고정, G1이 확인): `domain/issueops` `TestOpencodeStandaloneServerRequiresServeAndStdio`(`…/bin/opencode serve --stdio --port 0`·공백 든 경로 `/My Apps/opencode serve --stdio` 허용; `serve --service`, TUI `opencode`, `opencode run …`, `opencode --prompt "x opencode serve --stdio"`, `opencode run "opencode serve --stdio"` 거부)와 `TestNormalizeNativeActorBindsOpencodeHostToOpencodeReceipt`(host `opencode` + zsh receipt 거부, 시작 시각 형식과 무관); `application/authority` `TestVerifyNativeRequiresStandaloneOpencodeServer`(표: standalone serve 허용, 부모가 살아 있는 TUI인 `serve --service` 거부, 선언 host `codex` + opencode `serve --service` receipt 거부, 프롬프트에 `opencode serve --stdio`가 든 TUI receipt 거부, argv inspector 없음 거부, 시작 시각 불일치 거부); `adapter/issueops` `TestNativeProcessInspectorOpencodeArgumentsMatchStartIdentity`; `executioncmd` `TestNativeSessionIdentityFromEnvSupportsOpencode`와 `TestNativeSessionIdentityFromEnvRejectsOmpAndOpencodeTogether`. `internal/architecture` 테스트가 새 import 없이 통과하는지는 G9가 확인한다.
  - Must NOT do: 다른 host의 receipt 규칙 변경, CLI 쪽에만 규칙 두기.
  - Recommended Agent: deep; Reason: core 인가 경계.
  - Parallelization: NO; Wave 1; Blocks T2–T5, T7, T9; Blocked By —
  - References: `internal/domain/issueops/native_actor.go:11-42`, `internal/application/authority/service.go:59,139,210-230`, `internal/port/authority/authority.go:22-24`, `internal/adapter/issueops/execution_lease_identity.go:17`, `internal/adapter/issueops/execution_process.go:45-105`, `cmd/issueops/issueopscli/executioncmd/execution.go:200-243,301-363`, 실측 사슬과 바이너리 문자열(Gap Analysis).
  - Acceptance Criteria: G1 met.
  - QA Scenarios:
    - Channel: shell. Steps: `go test ./internal/domain/nativehost ./internal/domain/issueops ./internal/application/authority ./internal/adapter/issueops ./cmd/issueops/issueopscli/executioncmd -run 'Opencode' -count=1 -v`. Expected: 위 이름의 테스트가 모두 `--- PASS`, exit 0. Evidence: `.issueops/evidence/task-1-receipt.txt`
    - Channel: shell. Steps: 실제 프로세스로 거부 경로 확인 — (1) `env -u ISSUEOPS_OMP_SESSION_ID OPENCODE_SESSION_ID=x ./bin/issueops execution whoami --json`(opencode 조상이 없는 일반 셸), (2) 같은 env로 `./bin/issueops mcp authorize --workspace-root "$PWD" --host opencode --session-id x --session-pid $PPID --session-started-at <ps lstart로 얻은 값> --session-executable <ps comm> --json`(zsh receipt). Expected: (1) `reusable opencode host process ancestry is unavailable` 오류, (2) `opencode session process receipt must be the opencode standalone server` 오류, 새 grant 파일 0. Evidence: `.issueops/evidence/task-1-receipt-error.txt`
  - Commit: NO(한 PR, 커밋은 T10 뒤 atomic-commit-push가 주제별로 나눈다); Files: 위 경로.

- [ ] 2. opencode plugin 생성기와 argv
  - What to do:
    - `internal/adapter/hostprotocol/opencode.go`: `OpencodeLifecyclePlugin(binPath string) string`. 출력은 import 없는 ESM 한 파일: `const harnessBin = …`, JSON contract 상수(`{"schema_version":1,"hook":{"root":"session-start","child":"subagent-start"},"cache_limit":512,"timeout_ms":10000,"warning":"issueops lifecycle hook failed"}`), `export default { id: "issueops", setup(ctx) { … } }`. setup은 `ctx.session.hook("context", async (event) => …)` 하나만 등록한다. 동작: `event.sessionID`로 캐시 조회 → 없으면 `ctx.session.get({sessionID})`(실패 시 루트로 간주)로 `parentID`와 세션 디렉터리를 확인 → repo는 세션 정보의 디렉터리 필드가 있으면 그것, 없으면 `ctx.location.directory` → 루트면 `hook session-start --repo <repo> --json`, 자식이면 `hook subagent-start --repo <repo> --json`을 stdin `{"agent_type": event.agent}`와 함께 `Bun.spawn`으로 실행. 제한 시간은 `setTimeout`으로 10초 뒤 `proc.kill()`하고 그 결과를 실패로 다룬다(Bun.spawn의 timeout 옵션에 기대지 않는다). stdout JSON의 `should_inject && compact`면 그 문자열, 아니면 빈 문자열을 캐시(실패도 빈 문자열로 캐시하고 `console.error(warning)` 한 번) → 캐시 값이 비어 있지 않으면 `event.system.push({type:"text", text})`. 캐시 크기가 512를 넘으면 `Map`의 첫 key를 지운다. 예외는 hook 밖으로 던지지 않는다.
    - `ctx.session.get`의 정확한 호출 모양, 반환 필드(`parentID`, 디렉터리 필드 이름)는 Round 2 실측(`ctx.session.get`이 자식에 `parentID`를 돌려줌)을 따른다. 구현 첫 단계에서 임시 XDG + mock provider로 같은 호출을 한 번 실행해 필드 이름을 확인하고 생성기와 goja mock에 같은 이름을 쓴다. 디렉터리 필드가 없으면 `ctx.location.directory`만 쓴다(두 경우 모두 위 동작이 정의돼 있다).
    - `argv.go`: `BuildInteractiveArgv`의 opencode 분기 → `["/usr/bin/env", "OPENCODE_CONFIG_CONTENT=" + json({"model": M}), EXE, "--standalone", "--auto", <extra…>, "--prompt", PROMPT]`. effort는 받지만 넣지 않는다(근거: root model variant 무시 실측). `validateInteractiveHost`는 EXE(argv[2])로 `ExecutableMatchesHost`를 검사한다. 모델이 비면 `OPENCODE_CONFIG_CONTENT` 없이 `[EXE, "--standalone", "--auto", …, "--prompt", PROMPT]`.
    - `role_agents.go`: `BuildPrintArgv("opencode", M, E)` → `["opencode", "run", "--standalone", "--model", M+("#"+E if E)]`. 주석에 "프롬프트는 표준 입력"을 적는다. `RoleAgentArgs`는 opencode에 nil(omo·omp와 같음).
    - `internal/adapter/gitworktree/provisioner.go:187-204` `workspaceRelaunchCommand`: opencode → `cd <landing> && opencode --standalone`, 오류 문구 갱신.
    - 테스트: goja로 생성 모듈을 실행하는 `opencode_test.go`(mock `ctx.session.hook`/`get`, mock `Bun.spawn`; 루트·자식·explore·실패·timeout·캐시 1회·상한 512 축출·예외 비전파), 변이 증명(omp_test.go:114-175 패턴), `argv_test.go`·`role_agents_test.go`·`access_test.go` 표에 opencode 행.
  - Must NOT do: `process.env` 쓰기, 이벤트 구독 루프, `node:` import(goja·Bun 모두에서 돌도록).
  - Recommended Agent: deep; Reason: 생성 JS와 테스트 하네스.
  - Parallelization: YES; Wave 2; Blocks T3, T5, T7, T9; Blocked By T1
  - References: `internal/adapter/hostprotocol/omp.go`, `omo.go:8-118`, `omp_test.go:13-175`, `argv.go:16-80`, `role_agents.go:91-152`; opencode context hook 이벤트 키 `agent, messages, model, options, sessionID, system, tools`(Round 2), `ctx.location = {directory, project, workspaceID?}`(Round 2).
  - Acceptance Criteria: G2 met.
  - QA Scenarios:
    - Channel: shell. Steps: `go test ./internal/adapter/hostprotocol ./internal/adapter/gitworktree -count=1 -v -run 'Opencode|Interactive|PrintArgv|Relaunch'`. Expected: exit 0, `TestGeneratedOpencodeLifecyclePluginExecutesMockContextHook` PASS. Evidence: `.issueops/evidence/task-2-plugin.txt`
    - Channel: shell. Steps: 같은 테스트의 실패 케이스(spawn reject, 비 JSON stdout, timeout)가 push 0회·예외 0회를 단언. Expected: `--- PASS: …Failure…`. Evidence: `.issueops/evidence/task-2-plugin-error.txt`
  - Commit: NO; Files: 위 경로.

- [ ] 3. opencode 설치 adapter, 활성화·native 검증, inspect
  - What to do:
    - `internal/adapter/installutil/json_config.go`: key 경로 일반화(위 재사용 절). 호출자 이동.
    - `internal/adapter/opencode/`: `Installer{deps, lifecycle func(string) string, configRoot func(home) string}`. `Install`: 스킬 링크(`<configRoot>/skills`), `<configRoot>/opencode.json`의 `mcp.servers.issueops` 병합, `<configRoot>/plugins/issueops.js`(0644), `configs/opencode/opencode.json`(project 템플릿, `mcp.servers.issueops_project` stdio), `configs/opencode/issueops.js`. 파일 mode는 extensionhost 선례(`internal/adapter/extensionhost/mcp.go:21-35`)를 그대로 따른다: `WriteJSONPlan(…, 0o600, …)`로 쓰고, HTTP(bearer 포함)이고 dry-run이 아니면 기존 파일도 0600으로 좁힌다. `--project-local`도 extensionhost(`install.go:80-89`)와 같게: HTTP면 `.opencode/opencode.json`의 `mcp.servers.issueops_project`를 지우고(secret을 프로젝트에 쓰지 않음), stdio면 그 항목을 쓴다. `opencode.jsonc` 처리: Gap "설정 파일" 절대로 문자열·escape를 인식하는 주석·trailing comma 제거 함수 `normalizeJSONC`로 읽어 `mcp.servers.issueops`가 있으면 오류 "opencode.jsonc defines mcp.servers.issueops and would shadow the managed entry; remove it from opencode.jsonc", 읽기·파싱 실패면 plan 경고 메시지만 남기고 계속한다.
    - 항목 모양: HTTP `{"type":"remote","url":URL,"headers":{"Authorization":"Bearer …","X-Issueops-Mcp-Catalog-Sha256":SHA},"oauth":false,"codemode":false}`; stdio `{"type":"local","command":[BIN,"mcp"],"environment":{"ISSUEOPS_ROOT":ROOT,"ISSUEOPS_MCP_CATALOG_SHA256":SHA},"codemode":false}`.
    - `activation.go`: MCP 항목 의미 비교 + plugin 바이트 일치. `nativeactivation` readback 순서에 `opencode\x00mcp`, `opencode\x00hooks`를 omp 뒤에 추가하고 개수 9→11, 오류 "exactly eleven".
    - composition: `cmd/issueops/issueopsapp/host_installers.go`에 `newOpencodeInstaller()`, `install_wiring.go`에서 `codex, claude, omo, omp, opencode, agy` 순서와 readback, `probe_native_wiring.go`·`nativeintegration/validator.go`에 `OpencodeLifecyclePlugin` 주입. configRoot는 root가 `XDG_CONFIG_HOME`(절대 경로일 때) 또는 `$HOME/.config`로 정해 installer·validator·inspect에 같은 함수를 넘긴다.
    - native integration 검증(`validation_native_integration*.go`): 필수 경로 `configs/opencode/{opencode.json,issueops.js}`, `<configRoot>/skills/*/SKILL.md`, opencode MCP 파서(`mcp.servers`), plugin 바이트 비교. install dry-run probe(`validation_install_dry_run_contract.go:8,69-77`): required host에 opencode, 금지 경로에 `$HOME/.config/opencode`, `$ROOT/.opencode`.
    - inspect(`internal/adapter/inspect/hosts.go`, `hostconfig.go`, `receipts.go`): spec 추가, `mcp.servers` reader, `hostBinaryNames["opencode"]="opencode"`, 버전 문자열 `opencode v2.0.32` 파싱.
    - 텍스트: `install.go:25`, `update_bootstrap.go:23`, `scripts/install-native.sh:36` "Codex/Claude/Omo/omp/opencode", `install_native_interactive.go:36` project-local 목록, `transaction.go:167`.
    - 테스트: `internal/adapter/opencode` 설치(새 파일·기존 키 보존·다른 plugin 바이트 보존·HTTP 0600·jsonc shadow 거부·jsonc 파싱 실패 경고·`normalizeJSONC`의 `"$schema": "https://…"`/trailing comma/문자열 안 `/*`·escape된 따옴표 보존·dry-run 무쓰기·HTTP/stdio 모양·project-local HTTP 제거), activation, host 개수·순서 테스트(`install_contract_matrix_test.go:664`, `install_instance_test.go:39,43`, `nativeactivation` 테스트, `inspect/hosts_test.go:80-81`), `host_json_test.go`.
  - Must NOT do: `opencode` 프로세스 실행, `opencode.jsonc`·`cli.json`·`tui.jsonc` 쓰기, extensionhost Spec 확장.
  - Recommended Agent: deep; Reason: 설치 트랜잭션·검증 다중 파일.
  - Parallelization: YES; Wave 3; Blocks T8, T9; Blocked By T1, T2
  - References: `internal/adapter/extensionhost/install.go:43-99`, `mcp.go:12-117`, `activation.go:11-48`, `internal/adapter/agy/*`, `cmd/issueops/issueopsapp/install_wiring.go:33-131`, `host_installers.go:15-90`, `internal/domain/nativeactivation/validation.go:41-60`, surface map §C·§D·§E.
  - Acceptance Criteria: G3, G4 met.
  - QA Scenarios:
    - Channel: shell. Steps: G4 명령(임시 HOME·XDG·state로 `install --dry-run --json --mcp-transport=stdio`). Expected: 출력에 kind `opencode_user_mcp_config`, `opencode_user_lifecycle_plugin`, `opencode_project_mcp_template`, `opencode_lifecycle_extension_template`, `.config/opencode/skills/` 링크, 그리고 임시 HOME 아래 파일 0개. Evidence: `.issueops/evidence/task-3-dry-run.txt`
    - Channel: shell. Steps: `go test ./internal/adapter/opencode -run 'Jsonc|Preserve' -count=1 -v`. Expected: shadow 거부와 보존 테스트 PASS. Evidence: `.issueops/evidence/task-3-install-error.txt`
  - Commit: NO; Files: 위 경로.

- [ ] 4. role model opencode 절
  - What to do: `internal/contract/agentmodel/types.go` Config에 `Opencode map[Role]Layer \`json:"opencode,omitempty"\``. `internal/domain/agentmodel/defaults.go`: configurableHosts에 `opencode`, ladder `{"none","minimal","low","medium","high","xhigh","max"}`, builtins `opencode/deepseek-v4.1-flash-free` — implement·plan-review·diff-review `max`, research `high`, reader-check `low`(사용자 `~/.local/state/opencode/model.json`이 이 모델·`max`를 쓰고 `/api/model`에서 variant `low,high,max` 확인). 새 집합 `modelDefinedEffortHosts = {"opencode"}`: 이 host는 docs-only override(`resolve.go:85-87`)와 escalation 상승(`resolve.go:49-51`)을 적용하지 않는다(variant가 모델마다 달라 존재하지 않는 값을 만들 수 있으므로). `resolve.go` hostLayers, `validate.go:48-50`, `cmd/issueops/modelcli/model.go` usage·switch·clone.
  - Must NOT do: 다른 host의 docs-only·escalation 동작 변경.
  - Recommended Agent: quick; Reason: 표 확장.
  - Parallelization: YES; Wave 2; Blocks T5, T8; Blocked By T1
  - References: `internal/domain/agentmodel/defaults.go:11-99`, `resolve.go:29-123`, surface map §F.
  - Acceptance Criteria: G5 met.
  - QA Scenarios:
    - Channel: shell. Steps: `go test ./internal/domain/agentmodel ./internal/application/agentmodel ./cmd/issueops/modelcli -count=1`. Expected: exit 0. Evidence: `.issueops/evidence/task-4-model.txt`
    - Channel: shell. Steps: 임시 XDG에서 `./bin/issueops model resolve --host opencode --role diff-review --tier docs-only --json`. Expected: `effort` `max`(docs-only로 `medium`이 되지 않음), `argv` `["opencode","run","--standalone","--model","opencode/deepseek-v4.1-flash-free#max"]`. Evidence: `.issueops/evidence/task-4-model-docsonly.txt`
  - Commit: NO; Files: 위 경로.

- [ ] 5. 인계 profile과 cmux
  - What to do: `internal/adapter/cmux/artifact_test.go:31-35` host 표에 opencode(그리고 빠진 omp) 행. `executioncmd/execution.go:66,111` `handoff-cmux --host` 목록. `skills/issueops/references/session-choice.md`: opencode 권한 플래그 `--auto`, `expected_owner_host` enum, Herdr 절에 opencode 경로 추가 — `herdr pane run "$PANE_ID" "$LAUNCH_COMMAND"`, `LAUNCH_COMMAND`는 `OPENCODE_CONFIG_CONTENT='{"model":"<implement model>"}' opencode --standalone --auto --prompt '<handoff>'`(각 값 shell-quote), 수신 확인은 herdr가 opencode TUI plugin으로 보고하는 `ses_…` id와 `execution whoami`. omp Herdr 행도 함께 채운다(현재 빠짐). `internal/adapter/issueops/issueops_skill_contract_test.go:227-241`의 플래그 목록에 opencode `--auto`·omp `--auto-approve`를 더한다.
  - Must NOT do: Herdr Go 연동 추가.
  - Recommended Agent: quick; Reason: 표·문서.
  - Parallelization: YES; Wave 3; Blocks T8; Blocked By T2, T4
  - References: `internal/adapter/cmux/artifact.go:76-152`, `session-choice.md:50-63,186-188,228,343-369`, herdr 0.9.5 `agent start --kind opencode`(Round 2).
  - Acceptance Criteria: G6 met.
  - QA Scenarios:
    - Channel: shell. Steps: `go test ./internal/adapter/cmux ./internal/adapter/issueops -run 'Launcher|PermissionBypass' -count=1`. Expected: exit 0. Evidence: `.issueops/evidence/task-5-cmux.txt`
    - Channel: shell. Steps: cmux launcher 테스트에서 opencode argv의 `exec '/usr/bin/env' 'OPENCODE_CONFIG_CONTENT=…' '/opt/native/opencode' '--standalone' '--auto' '--prompt' "$prompt"` 줄을 단언. Expected: PASS. Evidence: `.issueops/evidence/task-5-cmux-argv.txt`
  - Commit: NO; Files: 위 경로.

- [ ] 6. trace `opencode-json`
  - What to do: `internal/contract/trace/host_usage.go`에 `InputFormatOpencodeJSON = "opencode-json"`. `internal/adapter/trace/host_usage.go`에 opencode collector: 줄마다 JSON, `type=="step_finish"`인 `part.tokens.input/output/reasoning/cache.read/cache.write`를 합산, `sessionID`·`timestamp`로 turn 구분, 비 JSON·다른 type은 무시. `internal/application/trace/service.go:37,88-92` 지원 목록, `cmd/issueops/basiccli/trace.go:32,93`, `internal/adapter/inbound/catalog/cli/usage.go:31`. fixture `internal/adapter/trace/testdata/host-usage/opencode-tool-turn.jsonl`은 T9 live 테스트의 mock 실행에서 얻은 실제 출력으로 만든다(round 2 실측 줄 `{input:1134, output:49, reasoning:7, cache:{read:100}}` 형식).
  - Must NOT do: 마지막 step 보정 추정.
  - Recommended Agent: quick; Reason: collector 하나.
  - Parallelization: YES; Wave 2; Blocks T8; Blocked By —
  - References: surface map §H, `internal/adapter/trace/host_usage.go:26-51`.
  - Acceptance Criteria: G7 met.
  - QA Scenarios:
    - Channel: shell. Steps: `go test ./internal/adapter/trace ./internal/application/trace ./cmd/issueops/basiccli -run 'Opencode|HostUsage' -count=1 -v`. Expected: opencode fixture 합계 단언 PASS. Evidence: `.issueops/evidence/task-6-trace.txt`
    - Channel: shell. Steps: 잘못된 줄이 섞인 입력에서 합계가 정상 줄만 반영되는 테스트. Expected: PASS. Evidence: `.issueops/evidence/task-6-trace-error.txt`
  - Commit: NO; Files: 위 경로.

- [ ] 7. host-probe opencode runner
  - What to do: `internal/adapter/hostprobe/opencode.go` `OpencodeRunner`(이름 `opencode`). preflight: `LookPath("opencode")`, 격리 env에서 `opencode --version`. 격리 env는 Gap "live 실행 env"의 허용 목록 + private `HOME`·`XDG_CONFIG_HOME/DATA/STATE/CACHE` + `OPENCODE_DISABLE_AUTOUPDATE=1`, `OPENCODE_DISABLE_MODELS_FETCH=1`, `OPENCODE_DISABLE_CLAUDE_CODE=1`, 무작위 `OPENCODE_PASSWORD`. explicit model 필수(`--model opencode=P/M`, 없으면 omp처럼 `not-run`). 자격 증명은 사용자 env의 provider 키 변수(이름이 `_API_KEY`로 끝나는 것)만 통과시키고 `opencode.db`는 읽지 않는다. episode 준비: private config dir에 `opencode.json` = `mcp.servers.issueops_probe`(stdio, `codemode:false`) + `permissions:[{"action":"*","resource":"*","effect":"deny"},{"action":"issueops_probe_*","resource":"*","effect":"allow"}]`(그 밖의 도구를 요청에서 제거해 전체 tool 수를 probe 도구로 제한, Round 4 실측), `plugins/issueops.js`(생성기 출력), `plugins/z-guard.js`(probe 전용 guard: 같은 context 이벤트에서 `issueops.js`가 push한 catalog 표지가 `event.system`에 있으면 세션을 injected로 기록하고, `ctx.tool.hook("execute.before")`에서 injected가 아닌 세션의 `issueops_probe_*` 호출을 throw로 막는다; plugin은 전체 경로 정렬로 `issueops.js` 다음에 실행됨, Round 4), 프로젝트 문서. 실행과 수명 주기: (1) `opencode serve --hostname 127.0.0.1 --port <빈 포트>`를 격리 env, cmd.Dir와 `PWD`=realpath episode dir(기본 location이 warm-up location과 같아지도록), 새 process group(`configureProcessGroup` 재사용)으로 시작하고, `server listening on` 줄을 읽은 뒤에도 stdout·stderr를 계속 비우며, 시작 직후 `defer`로 정리 함수를 등록한다 — 모든 반환 경로(준비 실패, warm-up 실패·timeout, MCP `failed`·`disabled`, run 오류, ctx 취소, 성공)에서 process group에 SIGTERM → 5초 대기 → SIGKILL → 상한 있는 `Wait`(기존 `awaitCommand`의 `processTerminationGrace`·`processTerminationHardLimit` 방식 또는 `cmd.WaitDelay`). (2) 준비는 `server listening on` 줄 대기와 `GET /api/info`(Basic `opencode:<password>`) 200 polling을 합쳐 상한 15초, 넘으면 실패. (3) warm-up은 `GET /api/mcp?location[directory]=<realpath episode dir>`를 200ms 간격, 상한 10초로 polling해 `issueops_probe`가 `connected`면 진행하고 `failed`·`disabled`면 즉시 실패(standalone 첫 요청에 MCP 도구가 빠지는 문제를 피함, Round 4·5). 두 상한은 runner 필드로 두어 테스트가 짧게 주입한다. (4) `opencode run --server http://127.0.0.1:<port> --model M --format json`을 cmd.Dir와 `PWD`=realpath episode dir, 프롬프트 표준 입력으로 기존 `CommandRunner`의 timeout 경로로 실행한다. 서버 시작과 HTTP 호출은 `Dependencies`의 주입점(`StartServer`(Cwd 포함 요청), `HTTPGet`)으로 둬 fake로 테스트한다. password는 env로만 넘기고 결과·evidence·오류 문구에 넣지 않는다. 결과 projection(`--format json` 줄): `tool_use` part의 tool 이름·입력·결과, 전체 tool 수, 마지막 `text`. `SessionStart=true`는 guard가 probe 호출을 허용해 실제 MCP 결과가 나온 경우에만 세우고, `PreToolUse=false`(PreToolUse hook 없음). `tool_conformance_facade.go:43-49`에 등록, `conformance_test.go:141-170` 기대 요청 수 3→4, facade 테스트 host 루프. 사용자 서비스(포트 49374)를 쓰는 명령(`opencode debug …`, `--server` 없는 `run`)은 쓰지 않는다.
  - Must NOT do: 기본 `--hosts` 변경, 실제 `~/.config/opencode`·`~/.local/share/opencode` 쓰기.
  - Recommended Agent: deep; Reason: 격리 실행 runner.
  - Parallelization: NO; Wave 4; Blocks T8; Blocked By T2, T3
  - References: `internal/adapter/hostprobe/omp.go:1-397`, `runner.go:39-113`, `run_finish.go:56-58,96-99`(guard 증거 선례), `omp_isolation_unix_test.go`·`runner_gating_test.go`(격리 테스트 선례), Round 4(tool hook·plugin 순서·permissions·tool 이름), Round 5(private serve·warm-up·`run --server`·종료).
  - Tests(이름 고정, G8이 확인): `TestOpencodeRunnerBuildsIsolatedEnvWithoutUserSessionIDs`(다른 host 세션 ID·테스트가 정하지 않은 `OPENCODE_*` 없음, 결과·evidence·오류에 password 문자열 없음), `TestOpencodeRunnerWarmsMCPBeforeRun`(fake serve·HTTP 표, 상한은 짧게 주입: 정상이면 serve Cwd가 episode dir이고 warm-up 뒤 run, 준비 상한 초과면 실패하고 정지 1회, warm-up 상한 초과면 실패하고 정지 1회, MCP `failed`·`disabled`면 즉시 실패하고 정지 1회, run 오류여도 정지 1회), `TestOpencodeGuardPluginBlocksProbeToolWithoutCatalog`(goja), `TestOpencodeRunnerProjectsRunJSONToolUse`, `TestOpencodeRunnerRequiresExplicitModel`, `TestOpencodeRunnerNeverWritesUserHome`(격리 HOME 쓰기 테스트, omp 선례).
  - Acceptance Criteria: G8 met.
  - QA Scenarios:
    - Channel: shell. Steps: `go test ./internal/adapter/hostprobe ./cmd/issueops/issueopsapp ./cmd/issueops/contractcli -run 'Opencode|Conformance|Facade' -count=1`. Expected: exit 0. Evidence: `.issueops/evidence/task-7-probe.txt`
    - Channel: shell. Steps: `ISSUEOPS_TOOL_CONFORMANCE_LIVE=1 ./bin/issueops contract conformance live --hosts opencode --model opencode=opencode/deepseek-v4.1-flash-free --target-completed 1 --json` 한 번 실행(네트워크·실모델). Expected: host evidence `installed:true`, `preflight_ready:true`, `live_attempted:true`; 모델 실패면 `status_reason`에 원인이 기록되고 사용자 `~/.local/share/opencode` mtime이 변하지 않음. Evidence: `.issueops/evidence/task-7-probe-live.json`
  - Commit: NO; Files: 위 경로.

- [ ] 8. MCP enum, usage, golden·inventory·템플릿 재생성
  - What to do: `internal/contract/mcp/issueops_catalog.go:19` enum에 `opencode`, `channel_catalog.go:9` 설명에 opencode, `internal/contract/cli/issueops_catalog.go:55,58,85,100-101`, `executioncmd/execution.go:66,68,111,158`, `remotecmd/remote.go:35-40`, `issueops_subcommands.go:467`, `mcp_authorize_command.go:43`, `modelcli` usage. 재생성: `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -update -count=1`, `go test ./internal/adapter -run TestNativeInstallAdapterContractMatrix -update-adapter-contract -count=1`, `go test ./internal/architecture -update-ddd-inventory -count=1`, `go test ./internal/adapter/opencode ./internal/adapter/extensionhost ./internal/adapter/agy -count=1`로 `configs/{omo,omp,agy,opencode}` 템플릿 갱신 확인(템플릿은 installer 테스트의 기대값과 같아야 한다; 다르면 installer 출력으로 덮는다). skill contract 문장 "First-party hosts are exactly Codex, Claude Code, Omo native, omp, and opencode."(`skill_contract_test.go:272,382`와 두 SKILL.md).
  - Must NOT do: golden 손편집.
  - Recommended Agent: quick; Reason: 재생성.
  - Parallelization: NO; Wave 5; Blocks T9, T10; Blocked By T3–T7
  - References: surface map §J·§K·§L·§M·§N.
  - Acceptance Criteria: G9 met.
  - QA Scenarios:
    - Channel: shell. Steps: `go test ./cmd/issueops/... ./internal/adapter ./internal/architecture -count=1`. Expected: exit 0. Evidence: `.issueops/evidence/task-8-golden.txt`
    - Channel: shell. Steps: `git diff --stat -- cmd/issueops/testdata internal/adapter/testdata configs`에서 다른 host 항목의 변화가 digest·순서 외에 없는지 `git diff`로 확인. Expected: codex/claude/omo/omp/agy 파일 경로·kind 변화 0. Evidence: `.issueops/evidence/task-8-golden-diff.txt`
  - Commit: NO; Files: golden·inventory·configs.

- [ ] 9. env-gated live 테스트(실제 opencode + mock provider)
  - What to do: `internal/adapter/opencode/live_test.go` `TestOpencodeLiveStandaloneSession`: `ISSUEOPS_OPENCODE_LIVE != "1"`이거나 `opencode`가 없으면 Skip. 준비: 임시 HOME·`XDG_CONFIG_HOME/DATA/STATE/CACHE`, 임시 `ISSUEOPS_STATE_DIR`, `go build`로 issueops 임시 바이너리, 그 바이너리를 감싸는 카운터 스크립트(인자 첫 단어가 `hook`이면 임시 로그 파일에 한 줄 쓰고 exec), `.issueops/CONSTITUTION.md`가 있는 임시 repo(git init), 같은 스킬 하나(`atomic-commit-push`)를 임시 `$HOME/.claude/skills`에도 symlink. 설치는 `opencode.Installer.Install`을 임시 Home, `BinPath`=카운터 스크립트, stdio transport로 직접 호출(사용자 홈·source bin 미변경). `httptest` OpenAI 호환 mock provider를 `OPENCODE_CONFIG_CONTENT`의 provider와 root `model`로 지정. 자식 env는 Gap "live 실행 env"의 허용 목록으로 새로 만든다(`OPENCODE_DISABLE_AUTOUPDATE=1`, `OPENCODE_DISABLE_MODELS_FETCH=1` 포함, 다른 host 세션 ID 없음). mock은 요청 본문에 `tools` 키가 있는 요청만 agent-loop 요청으로 세고, 제목 생성 요청(`tools` 없음, Round 2 실측)에는 짧은 텍스트로 답하며 번호에서 뺀다(구현 첫 단계에서 mock 로그로 이 판별을 확인한다). agent-loop 응답 순서: 1번째 → shell tool call `sleep 1; <임시 바이너리 절대 경로> execution whoami --json`(stdio MCP 연결 시간을 확보, Round 5에서 0.05~1.2초), 2번째 → 직전 tool 결과(whoami JSON)의 `claim_actor_flags[0]`을 붙인 shell tool call `<임시 바이너리> mcp authorize --workspace-root <repo> <claim_actor_flags[0]> --json`(`mcp authorize`는 actor 플래그가 필수, `cmd/issueops/issueopsapp/mcp_authorize_command.go:57-69`), 3번째 → 요청 `tools`에 `issueops_docs_index`가 있으면 그 MCP tool call, 없으면 `sleep 1` shell call을 최대 3번까지 끼워 넣은 뒤 다시 확인(그래도 없으면 테스트 실패), 그다음 → 텍스트로 종료. `opencode run --standalone --format json`을 cwd=repo, `PWD`=repo, 프롬프트 표준 입력으로 실행(타임아웃 120초). 단언: (a) 모든 agent-loop 요청의 system에 catalog 표지(`.issueops/CONSTITUTION.md` 경로 문자열)가 있다, (b) MCP tool call을 보낸 agent-loop 요청 `tools`에 `issueops_harness_inspect`와 `issueops_docs_index`가 있다(standalone 첫 요청은 stdio MCP 연결 전에 나가 MCP 도구가 없을 수 있다, Round 4·5 실측), (c) 1번째 agent-loop 요청 본문(system 또는 skill 도구 설명 — 구현 첫 단계에서 위치와 항목 표기를 확인)에서 `atomic-commit-push` 스킬 **항목**(목록 항목 단위, 경로 문자열 중복은 세지 않음)이 정확히 하나다, (d) 2번째 agent-loop 요청의 tool 결과가 whoami JSON으로 `host:"opencode"`, `session_id_source:"OPENCODE_SESSION_ID"`, ancestry[0] basename `opencode`, (e) 3번째 agent-loop 요청의 tool 결과가 `ok:true`와 `authority_file`을 가진다(claim과 같은 verifier가 standalone `serve --stdio` receipt를 받아들임), (f) MCP tool call 다음 요청의 tool 결과가 docs_index 결과(`docs` 배열)다, (g) 카운터 로그의 `hook` 줄이 정확히 1개다(세션당 한 번 실행, 여러 번의 모델 호출). 실행 stdout은 `-fixture-out` 플래그가 있을 때만 T6 fixture로 저장한다.
  - Must NOT do: 사용자 서비스 접속(`--standalone` 필수), 실모델 호출.
  - Recommended Agent: deep; Reason: 실제 host 통합.
  - Parallelization: NO; Wave 5; Blocks T10; Blocked By T3, T8
  - References: Round 2·3 mock provider 실험 결과, `internal/adapter/issueops/issueops_cleanup_linked_branch_live_test.go:29-33`(env gate 선례).
  - Acceptance Criteria: G10 met.
  - QA Scenarios:
    - Channel: shell. Steps: G10 명령. Expected: `OPENCODE_LIVE_PASS`. Evidence: 원장 EVIDENCE
    - Channel: shell. Steps: 같은 테스트에서 공유 서비스 사슬을 흉내 낸 ancestry로 `NormalizeNativeActor`가 거부되는 단위 테스트(T1) 결과를 함께 보고. Expected: T1 증거 참조. Evidence: `.issueops/evidence/task-9-live-error.txt`
  - Commit: NO; Files: `internal/adapter/opencode/live_test.go`, `internal/adapter/trace/testdata/host-usage/opencode-tool-turn.jsonl`

- [ ] 10. ADR, 문서, 스킬, 이슈 동기화
  - What to do: `project_docs_append(kind=adr)`로 `adr/2026-10-18-opencode-first-party-host.md`(결정: 다섯 번째 host, 설치 경로, plugin context hook, `OPENCODE_SESSION_ID`, argv 기반 standalone receipt 규칙(적용 범위는 opencode 실행 파일 receipt와 선언 host `opencode`뿐이며 다른 host의 receipt 결합은 기존 동작 그대로)과 그 결과(공유 서비스 세션의 lease·`mcp authorize` 거부와 transport별로 남는 MCP 도구, standalone 첫 요청에 MCP 도구가 없을 수 있음), model-defined effort, 대화형 실행의 effort 미적용(TUI가 사용자 `model.json` variant를 쓰므로 prepare가 보고한 effort와 다를 수 있음), print stdin, trace 한계, probe 자격 증명·autoupdate 정책과 warm-up, cmux 미실측, 이전 바이너리 비호환). `ADR.md` baseline 문장·색인, `TECH_STACK.md` goja 용도. surface map §O 문서(`AGENTS.md` "네 호스트"·smoke 경로, `README.md`·`README.en.md`, `.issueops/OPERATIONS.md`, `operations/install.md`(jsonc shadow 복구 절차 포함), `operations/verification.md:33`, `operations/guides/hosts.md`에 `## opencode` 절, `cli-and-mcp.md`, `issueops-execution.md` 모델 표, `skills-and-hosts.md`, `architecture/host-integration.md` 표 행, `hexagonal-core.md`, `runtime.md`, `domain-responsibilities.md`, `CONVENTIONS.md:76`, `conventions/*`, `TESTING.md:18`, `testing/*`, `AGENT_WORKFLOW.md:71`, `CAUTIONS.md:43-46`)와 §P 스킬(`io-update`, `io-model`, `self-verify`, `verified-execution`, `self-augment`, `issueops/references/execution.md`). 이슈 본문은 인계 전에 기록한 contract_change feedback에 따라 `issueops-sync-issue`로 갱신하고 `feedback mark-issue-updated`를 기록한다(아직 끝나지 않았으면 이 단계에서 마무리).
  - Must NOT do: 날짜 지난 ADR 본문 수정(새 ADR이 supersede 표시만).
  - Recommended Agent: quick; Reason: 문서.
  - Parallelization: NO; Wave 6; Blocks —; Blocked By T8, T9
  - References: surface map §O·§P, `project-docs-optimize` checker.
  - Acceptance Criteria: G9(스킬 문장), self-verify docs step.
  - QA Scenarios:
    - Channel: shell. Steps: `python3 skills/project-docs-optimize/scripts/check.py --mode check` 와 `./bin/issueops docs --json`. Expected: 문제 0, docs 목록에 새 ADR. Evidence: `.issueops/evidence/task-10-docs.txt`
    - Channel: shell. Steps: `git grep -nE 'Codex/Claude/Omo/omp([^/]|$)|Omo, and omp|four hosts|네 호스트|omo, or omp' -- ':!.issueops/issues' ':!.issueops/adr/2026-0*' ':!.issueops/adr/2026-10-0*' ':!.issueops/adr/2026-10-1[0-5]*' ':!.issueops/research' ':!.issueops/plans' ':!.issueops/verified-execution' ':!.issueops/archive' ':!docs/research'`. Expected: 0줄(날짜 지난 ADR·연구 기록은 제외). Evidence: `.issueops/evidence/task-10-docs-error.txt`
  - Commit: NO; Files: 문서·스킬.

## 게이트

- G1: opencode actor는 `serve --stdio` receipt로만 허용되고, 부모가 살아 있는 공유 서비스·host를 바꾼 플래그·오래 사는 조상 receipt는 core verifier에서 거부된다 | CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','-run','Opencode','./internal/domain/nativehost','./internal/domain/issueops','./internal/application/authority','./internal/adapter/issueops','./cmd/issueops/issueopscli/executioncmd'],capture_output=True,text=True);o=p.stdout+p.stderr;need=['TestOpencodeStandaloneServerRequiresServeAndStdio','TestNormalizeNativeActorBindsOpencodeHostToOpencodeReceipt','TestVerifyNativeRequiresStandaloneOpencodeServer','TestNativeProcessInspectorOpencodeArgumentsMatchStartIdentity','TestNativeSessionIdentityFromEnvSupportsOpencode','TestNativeSessionIdentityFromEnvRejectsOmpAndOpencodeTogether'];m=[n for n in need if '--- PASS: '+n not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m))" | EXPECT: ALL_PASS
- G2: 생성 plugin이 mock opencode 런타임에서 세션당 한 번 catalog를 읽어 매 호출 system에 넣고 실패를 삼키며, opencode argv가 설치된 플래그만 쓴다 | CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','-run','Opencode','./internal/adapter/hostprotocol','./internal/adapter/gitworktree'],capture_output=True,text=True);o=p.stdout+p.stderr;need=['TestGeneratedOpencodeLifecyclePluginExecutesMockContextHook','TestGeneratedOpencodeLifecyclePluginProofRejectsRuntimeMutations','TestBuildInteractiveArgvOpencodeStandalone','TestBuildPrintArgvOpencodeRun','TestWorkspaceRelaunchCommandOpencode'];m=[n for n in need if '--- PASS: '+n not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m))" | EXPECT: ALL_PASS
- G3: opencode 설치 adapter와 활성화·inspect·native 검증 테스트가 통과한다 | CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','./internal/adapter/opencode','./internal/adapter/installutil','./internal/adapter','./internal/adapter/inspect','./internal/adapter/verification/...','./internal/domain/nativeactivation','./internal/application/nativeactivation','./cmd/issueops/issueopsapp','./cmd/issueops/installcli'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL'+chr(10)+(p.stdout+p.stderr)[-2500:])" | EXPECT: ALL_PASS
- G4: 새로 빌드한 바이너리의 임시 HOME install dry-run이 opencode 설정·plugin·스킬 링크를 계획하고 임시 HOME에 파일·디렉터리·링크를 하나도 만들지 않는다 | CHECK: python3 -c "import subprocess as s,tempfile,os,json;b=s.run(['go','build','-o','bin/issueops','./cmd/issueops'],capture_output=True,text=True);h=tempfile.mkdtemp();st=tempfile.mkdtemp();e=dict(os.environ,HOME=h,ISSUEOPS_STATE_DIR=st);e.pop('XDG_CONFIG_HOME',None);p=s.run(['./bin/issueops','install','--dry-run','--json','--mcp-transport=stdio'],capture_output=True,text=True,env=e);o=p.stdout;k=['opencode_user_mcp_config','opencode_user_lifecycle_plugin','opencode_project_mcp_template','opencode_lifecycle_extension_template','/.config/opencode/skills/'];m=[x for x in k if x not in o];w=os.listdir(h);print('DRY_RUN_OK' if b.returncode==0 and p.returncode==0 and not m and not w else 'FAIL build=%d rc=%d missing=%s written=%s'%(b.returncode,p.returncode,m,w[:5]))" | EXPECT: DRY_RUN_OK
- G5: opencode role model이 설정 파일·builtin을 따르고 docs-only·escalation이 variant를 바꾸지 않는다 | CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','-run','Opencode','./internal/domain/agentmodel','./internal/application/agentmodel','./cmd/issueops/modelcli'],capture_output=True,text=True);o=p.stdout+p.stderr;need=['TestResolveOpencodeKeepsModelDefinedEffort'];m=[n for n in need if '--- PASS: '+n not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m))" | EXPECT: ALL_PASS
- G6: cmux launcher와 인계 스킬이 opencode를 standalone·auto로 연다 | CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','./internal/adapter/cmux','./internal/adapter/issueops','./cmd/issueops/issueopscli/executioncmd'],capture_output=True,text=True);t=open('skills/issueops/references/session-choice.md').read();ok=p.returncode==0 and 'opencode --standalone --auto' in t;print('ALL_PASS' if ok else 'FAIL rc=%d'%p.returncode)" | EXPECT: ALL_PASS
- G7: trace opencode-json이 step_finish 토큰을 합산한다 | CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','-run','Opencode','./internal/adapter/trace','./internal/application/trace','./cmd/issueops/basiccli'],capture_output=True,text=True);o=p.stdout+p.stderr;print('ALL_PASS' if p.returncode==0 and '--- PASS: TestOpencodeHostUsage' in o else 'FAIL rc=%d'%p.returncode)" | EXPECT: ALL_PASS
- G8: hostprobe opencode runner가 사용자 세션 ID 없는 격리 env와 warm-up한 private serve로 episode를 구성하고, catalog 주입 없이는 guard가 probe 도구를 막으며, conformance가 opt-in일 때만 opencode를 요청한다 | CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','./internal/adapter/hostprobe','./cmd/issueops/contractcli','./cmd/issueops/issueopsapp'],capture_output=True,text=True);o=p.stdout+p.stderr;need=['TestOpencodeRunnerBuildsIsolatedEnvWithoutUserSessionIDs','TestOpencodeRunnerWarmsMCPBeforeRun','TestOpencodeGuardPluginBlocksProbeToolWithoutCatalog','TestOpencodeRunnerProjectsRunJSONToolUse','TestOpencodeRunnerRequiresExplicitModel','TestOpencodeRunnerNeverWritesUserHome'];m=[n for n in need if '--- PASS: '+n not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m))" | EXPECT: ALL_PASS
- G9: build·vet·gofmt·전체 race 테스트가 통과하고 MCP enum이 opencode를 포함한다(golden·inventory·템플릿 일치는 이 테스트들이 확인한다) | CHECK: python3 -c "import subprocess as s;r=[s.run(c,capture_output=True,text=True) for c in (['go','build','-o','bin/issueops','./cmd/issueops'],['go','vet','./...'],['go','test','-race','-count=1','./...'])];f=s.run(['gofmt','-l','cmd','internal'],capture_output=True,text=True).stdout.strip();t=open('internal/contract/mcp/issueops_catalog.go').read();ok=all(x.returncode==0 for x in r) and not f and '\"opencode\"' in t;print('ALL_PASS' if ok else 'FAIL gofmt=%r rcs=%s'%(f[:200],[x.returncode for x in r]))" | EXPECT: ALL_PASS
- G10: 실제 opencode 2.0.32 standalone 세션이 catalog를 받고 issueops MCP 도구를 보며 whoami가 standalone receipt를 낸다 | CHECK: python3 -c "import subprocess as s,os;e=dict(os.environ,ISSUEOPS_OPENCODE_LIVE='1');p=s.run(['go','test','-count=1','-v','-run','TestOpencodeLiveStandaloneSession','./internal/adapter/opencode'],capture_output=True,text=True,env=e);o=p.stdout+p.stderr;print('OPENCODE_LIVE_PASS' if p.returncode==0 and '--- PASS: TestOpencodeLiveStandaloneSession' in o and '--- SKIP' not in o else 'FAIL rc=%d'%p.returncode+chr(10)+o[-2500:])" | EXPECT: OPENCODE_LIVE_PASS
- G11: CI와 같은 격리 환경(임시 clone, 임시 HOME·XDG·state, stdio 설치)에서 self-verify가 종료 조건을 만족한다 | CHECK: python3 -c "import subprocess as s,tempfile,os,shutil,json;t=tempfile.mkdtemp();r=os.path.join(t,'r');h=os.path.join(t,'h');os.makedirs(h);s.run(['git','clone','-q','.',r],check=True);L=lambda *a:s.run(['git','ls-files','-z',*a],capture_output=True,text=True).stdout.split(chr(0));D=lambda f:os.path.join(r,f);rm=lambda p:os.path.lexists(p) and os.remove(p);[(os.makedirs(os.path.dirname(D(f)),exist_ok=True),rm(D(f)),shutil.copy2(f,D(f),follow_symlinks=False)) for f in L('-co','--exclude-standard') if f and os.path.lexists(f)];[rm(D(f)) for f in L('-d') if f];e=dict(os.environ,HOME=h,CODEX_HOME=h+'/.codex',XDG_CONFIG_HOME=h+'/.config',ISSUEOPS_STATE_DIR=h+'/state');e.pop('ISSUEOPS_ROOT',None);b=s.run(['./scripts/install-native.sh','--path-mode=skip','--mcp-transport=stdio'],cwd=r,env=e,capture_output=True,text=True);v=s.run(['./bin/issueops','self-verify','--seed=100','--target-score=95','--llm-eval=false','--json'],cwd=r,env=e,capture_output=True,text=True);d=json.loads(v.stdout or '{}');print('ISOLATED_SELF_VERIFY_OK' if b.returncode==0 and d.get('ok') and d.get('termination_eligible') else 'FAIL install=%d verify=%d'%(b.returncode,v.returncode))" | EXPECT: ISOLATED_SELF_VERIFY_OK

## Final Verification Wave
- F1 Plan Compliance Audit: 모든 TODO가 지정대로 실행됐는지 원장·diff로 대조.
- F2 Code Quality Review: issueops-slop-clean 단계와 issueops-review(diff)로 AI slop·dead code·과한 추상화 확인.
- F3 Real Manual QA: G4·G10 실측과 T7 live episode 증거.
- F4 Scope Fidelity Check: Must NOT Have 위반 0(`git grep -n 'process.env\[' configs/opencode`, `opencode.jsonc` 쓰기 코드 0).

## Commit Strategy
`atomic-commit-push`로 주제별 Conventional Commit + Lore body. 예정 단위: (1) `feat(host): recognize opencode sessions and require a standalone receipt`, (2) `feat(install): install the opencode MCP entry, plugin, and skills`, (3) `feat(model): add opencode role models and launch profiles`, (4) `feat(trace,probe): read opencode run JSON and probe opencode episodes`, (5) `test(contract): regenerate goldens and templates for the opencode host`, (6) `docs: record opencode as the fifth first-party host`. 단위 경계가 테스트를 깨면 합친다.

## Success Criteria
- G1–G11 met(G11이 격리 self-verify `termination_eligible: true`), draft PR 발행과 `execution complete`.
