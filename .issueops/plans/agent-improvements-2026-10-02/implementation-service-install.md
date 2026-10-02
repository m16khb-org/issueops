# J 구현 보고: MCP 서비스 supervisor와 HTTP install/update/bootstrap

범위: contract.md §3(transport·설치·수명 중 J 몫), §5 J 행, §6의 서비스·설치 acceptance다.
inspect 진단(K), 구조화 결과(L), golden·inventory·운영 문서(Z)는 구현하지 않았다.
commit, 전역 설치, 실제 `$HOME` 설정 변경은 하지 않았다. 실측은 모두 `/tmp/jsvcmur1hqkw` 아래 격리 HOME·CODEX_HOME·state·root 사본과 테스트 전용 launchd 라벨로 했다.

## 결과 요약

- `issueops mcp service start|stop|status --json`이 실제 supervisor를 제어한다. `newMCPService()`는 이제 `internal/adapter/mcpservice.Service`를 반환한다. darwin에서는 LaunchAgent `io.issueops.mcp`(`$HOME/Library/LaunchAgents/io.issueops.mcp.plist`), linux에서는 systemd user unit `issueops-mcp.service`(`$HOME/.config/systemd/user/`)를 쓴다.
- unit은 절대 binary 경로와 `mcp --http`를 실행하고, 환경 변수는 `ISSUEOPS_ROOT`와 `ISSUEOPS_STATE_DIR` 두 개만 둔다. unit 파일에는 bearer가 없다.
- 서버(`issueops mcp --http`)는 listen 전에 `<state>/mcp-http/server.lock`에 fcntl write lock을 non-blocking으로 잡는다. 이미 잡혀 있으면 `conflict`로 종료하고, 포트가 사용 중이어도 `conflict`로 종료한다. 다른 포트로 바꾸지 않는다.
- listen에 성공하면 서버가 `server.json`(pid, started_at, executable, build_id=실행 파일 SHA-256)을 0600으로 기록하고, 인증에 성공한 2xx 응답에만 `Issueops-Mcp-Pid`·`Issueops-Mcp-Build-Id` 헤더를 붙인다. 401·403 응답에는 이 헤더가 없다.
- `running` 판정에는 다섯 가지가 모두 일치해야 한다. lock holder pid(F_GETLK로 lock을 잡지 않고 관측), record의 pid, OS에서 관측한 started_at·executable, bearer로 인증한 `initialize` 응답의 pid·build_id다. 하나라도 어긋나면 `conflict`(`identity_mismatch`)다. lock holder가 없는데 record가 남아 있으면 `stale`(`stale_record`)이고, lock holder는 있지만 응답이 없으면 `stale`(`not_ready`)이다.
- stop은 위 신원이 모두 일치하고, supervisor가 관리하는 pid(`launchctl print`의 `pid`, systemd `MainPID`)가 같을 때만 bootout/stop한다. 그 뒤 lock이 풀릴 때까지 기다린다. foreground 서버와 신원이 맞지 않는 인스턴스는 건드리지 않는다.
- supervisor가 없는 OS는 `supervisor_unsupported`, launchctl·systemctl을 찾지 못하면 `supervisor_unavailable` 오류다. stdio로 조용히 전환하지 않는다.
- install에 `--mcp-transport=http|stdio`를 추가했다. 기본값은 darwin/linux에서 http, 그 밖에서는 stdio다. update/bootstrap도 같은 플래그를 받아 `scripts/install-native.sh`에 넘긴다.
- http 설치는 Codex `[mcp_servers.issueops]`, Claude `~/.claude.json`, Omo `~/.omo/mcp.json`의 issueops entry만 교체한다. stdio의 command/args/env는 사라지고 파일은 0600이 된다. 기존 파일이 0644여도 bearer를 쓰기 전에 0600으로 바꾼다. agy는 stdio를 유지한다.
- `--project-local`과 http를 함께 쓰면 project entry를 만들지 않고 secret도 저장하지 않는다. 기존 `issueops_project` stdio entry만 제거하고, 같은 파일의 다른 entry는 보존한다.

## 설치·갱신 순서

| 단계 | 실행 주체 | 동작 |
|---|---|---|
| build | `install-native.sh` | staged binary 빌드, `version`, `install --dry-run` preflight |
| credential·unit | staged binary `install`(STEP=begin) | `Prepare`: bearer 확보(`EnsureHTTPBearer`), 0600 log 파일, unit 파일 기록. 로드하지 않는다 |
| service stop | staged binary `mcp service stop` | 신원 확인 뒤 bootout, lock 해제까지 대기 |
| binary 교체 | `install-native.sh` | 기존 원자적 `os.replace` |
| service start | 새 binary `mcp service start` | bootstrap, 인증된 `initialize`로 readiness 확인, build_id가 새 binary SHA-256과 같아야 성공. 실패하면 job을 내린다 |
| 확인 → host merge | 새 binary `install`(STEP=seal) | `EnsureRunning`이 build_id·인증 응답을 다시 확인한 뒤에만 `RunTransaction`이 host 설정을 쓰고 activation을 seal한다 |

contract는 "credential/unit → build" 순서를 적었지만, 준비 코드가 새 build 안에 있기 때문에 build 직후·stop 전에 실행한다. 이 위치에서도 서비스를 멈추거나 binary를 바꾸기 전에 credential·unit 실패로 중단한다는 목적은 지켜진다. 어느 단계든 실패하면 스크립트의 기존 `cleanup_install`이 activation을 abort하고, seal 전이므로 host 설정은 바뀌지 않는다. 직접 `issueops install`(step 없음)을 실행할 때도 `EnsureRunning`(필요하면 신원 확인 stop → start)이 host 쓰기보다 먼저 실행된다. dry-run은 `ReadBearer`(읽기 전용)만 호출하므로 파일·프로세스·credential을 만들지 않는다.

## 변경 파일

| 경로 | 내용 |
|---|---|
| `internal/adapter/mcpservice/instance.go` (신규) | `AcquireInstance`/`Publish`/`Release`, `IdentityHandler`, `BuildID`, record 읽기·쓰기, 읽기 전용 `ReadBearer` |
| `internal/adapter/mcpservice/lock_unix.go`, `lock_other.go` (신규) | fcntl `F_SETLK`/`F_GETLK` lock과 holder 관측. 그 밖의 OS는 명시적 미지원 |
| `internal/adapter/mcpservice/supervisor.go` (신규) | launchd plist·systemd unit 렌더링(XML escape, systemd `%`·`$` escape), load/unload/supervised pid |
| `internal/adapter/mcpservice/service.go` (신규) | `Status`/`Start`/`Stop`(port.MCPService), `Prepare`, `EnsureRunning`, 상태 분류와 오류 코드 |
| `cmd/issueops/issueopsapp/mcp_service_wiring.go` (신규) | 조립: 안정 root의 `bin/issueops`, 절대 state 경로, `ps` 기반 process inspector, `EnsureHTTPBearer`, 고정 주소. 격리 검증용 `ISSUEOPS_MCP_SERVICE_LABEL` 라벨 override |
| `cmd/issueops/issueopsapp/mcp_service_command.go` | 기본 `unavailableMCPService`를 지우고 `newMCPService()`가 supervisor adapter를 반환한다. `Run`은 바꾸지 않았다 |
| `cmd/issueops/issueopsapp/mcp_http_command.go` | `serveMCPHTTP`: bearer 확보 → instance lock → listen → identity record 기록 → identity header handler. 종료할 때 자기 record만 지우고 lock을 푼다 |
| `cmd/issueops/issueopsapp/install_wiring.go` | `DefaultMCPTransport`, `MCPURL`, `MCPService` 주입 |
| `cmd/issueops/issueopsapp/host_installers.go` | Claude/Omo에 `RemoveJSONMapEntry` 주입 |
| `cmd/issueops/installcli/install_mcp_transport.go` (신규), `install.go`, `dependencies.go` | `--mcp-transport`, transport 해석, 단계별 `applyMCPTransport` |
| `cmd/issueops/updatecli/update_bootstrap.go`, `internal/application/update/service.go` | update/bootstrap의 `--mcp-transport` 전달 |
| `scripts/install-native.sh` | `--mcp-transport=` 해석, 기본값, 교체 전 stop과 교체 후 start, `--mcp-transport`(공백 형식) 거부 |
| `internal/adapter/codex/{install_config,activation}.go` | http block(`url`, `http_headers`), 쓰기 전 0600, http readback(env section 0개) |
| `internal/adapter/claude/{install,install_mcp,dependencies}.go` | http entry, 0600, project-local http의 중복 entry 제거 |
| `internal/adapter/omo/{install,mcp,dependencies}.go` | 위와 같다(Omo) |
| `internal/adapter/installutil/json_config.go` | `RemoveJSONMapEntry` |

테스트: `internal/adapter/mcpservice/{service,supervisor,lock_unix}_test.go`, `internal/adapter/{codex,claude,omo}/install_http_test.go`, `cmd/issueops/installcli/install_mcp_transport_test.go`, `cmd/issueops/issueopsapp/mcp_service_wiring_test.go`, `internal/application/update/service_test.go`(케이스 1개 추가). H의 `TestMCPServiceDefaultReportsMissingSupervisorWithoutClaimingState`는 제거한 기본 구현을 고정하던 테스트라서 지우고 조립 테스트로 대체했다. Claude/Omo test fixture에는 새 의존성 한 줄씩을 추가했다.

`configs/` template은 바꾸지 않았다. template은 repo 안 `./bin/issueops`를 stdio로 실행하는 project-local 예시이고, http에서는 project entry를 만들지 않는다는 contract와 충돌하지 않는다.

## RED → GREEN

| 테스트 | RED | GREEN |
|---|---|---|
| `TestCodexHTTPInstallReplacesStdioEntryWithOwnerOnlyURLAndBearer` | 구현 전: 설정에 stdio block이 그대로 기록됨 | 정확한 TOML 일치, 0600, readback 통과, 다른 bearer는 거부하며 오류에 bearer 없음 |
| `TestClaudeHTTPInstallMergesOnlyIssueOpsEntryOwnerOnly` | 구현 전: `type:"stdio"`, command/env가 기록됨 | 정확한 JSON 일치, 0600, `issueops_project`만 제거, readback 통과 |
| `TestOmoHTTPInstallMergesOnlyIssueOpsEntryOwnerOnly` | 구현 전: command/env와 catalog env가 기록됨 | 위와 같다 |
| `{Codex,Claude,Omo}HTTPDryRunWritesNothing` | 구현 전부터 통과(회귀 고정용) | 통과 |
| mcpservice 16개(상태 분류, 2회 start, 포트 충돌, stale, readiness timeout, 신원 불일치, foreground 거부, supervised pid, EnsureRunning, unsupported·unavailable, 헤더, ReadBearer, 프로세스 간 lock, unit 렌더링) | 패키지가 없어 컴파일 실패. 첫 실행에서 fake endpoint가 항상 열려 있어 2건 실패했고, 원인은 테스트 fake라서 serving 중에만 listen하도록 고쳤다 | `go test -race` 통과 |
| installcli 4개(dry-run은 읽기만, begin은 prepare만, 서비스 실패 시 host installer 미호출, stdio는 서비스 미접촉) | `Deps` 필드가 없어 컴파일 실패 | 통과 |

시간 의존은 없다. lock 테스트는 helper 프로세스가 `LOCKED`를 출력한 뒤에만 경쟁 동작을 시작하고, 대기 상한은 `time.After(30s)` 하나다. readiness와 stop 대기는 fake `Wait`로 횟수를 제한한다.

## 명령 결과

| 명령 | 결과 |
|---|---|
| `gofmt -l`(추적 파일과 새 Go 파일 전체) | 출력 없음 |
| `go vet ./...` | 출력 없음, exit 0 |
| `go build -o /tmp/jsvc-build/issueops ./cmd/issueops` | exit 0. 실제 host가 stdio로 쓰는 repo `bin/issueops`는 바꾸지 않았다 |
| `GOOS=linux go vet`, `GOOS=windows go vet` (`./internal/adapter/mcpservice/`) | exit 0. build tag별 lock 구현이 컴파일된다 |
| `go test ./... -count=1` | 실패는 2건이고 둘 다 J 범위 밖의 기존 RED다. `TestResponseContractsGolden`은 `$.cli.self_verify_compare.baseline_step_duration_stats[0].reused_count unexpected`인데, H 보고서와 같은 B 소유 경로다. `TestDDDResponsibilityInventoryMatchesSource`는 Z 소유이며 J의 신규 파일도 등록 대상이다. 나머지 패키지는 모두 ok다. `internal/architecture`의 다른 테스트(orphan package, dependency 방향)도 통과했다 |
| `go test -race -count=1` mcpservice, installcli, codex, claude, omo, installutil, application/update, updatecli | 8개 패키지 모두 ok |
| `go test -count=1 ./cmd/issueops/issueopsapp/ -run 'Install\|Host\|Native\|MCP\|Supervisor'` | ok. H의 HTTP 테스트(바쁜 주소의 conflict 포함)도 instance lock을 추가한 뒤 그대로 통과한다 |
| `bash -n scripts/install-native.sh` | exit 0 |

## 실제 격리 실측

환경: `HOME=/tmp/jsvcmur1hqkw/home`, `CODEX_HOME=$HOME/.codex`, `ISSUEOPS_STATE_DIR=/tmp/jsvcmur1hqkw/state`, `ISSUEOPS_ROOT=/tmp/jsvcmur1hqkw/root`(현재 작업 트리를 `.git` 없이 rsync한 사본, `configs/upstream.json`은 네트워크 설치를 막으려고 빈 목록으로 바꿨다), `ISSUEOPS_MCP_SERVICE_LABEL=io.issueops.mcp.jtest-jsvcmur1hqkw`. Go 빌드는 기존 module·build cache를 쓰고 `GOPROXY=off`로 네트워크를 막았다. 시작 전에 47831 포트는 비어 있었고 실제 `io.issueops.mcp` 라벨은 로드되지 않은 상태(`launchctl print` exit 113)였다.

### 서비스 수명

| 명령 | 결과 |
|---|---|
| `mcp service status --json` | `{"ok":true,"status":"stopped","pid":0,...,"error_code":""}` |
| `mcp service start --json` | `running`, pid 56013, build_id `ab1cad68…3b95`(= `shasum -a 256 root/bin/issueops`) |
| `plutil -p <temp plist>` | ProgramArguments `[/tmp/jsvcmur1hqkw/root/bin/issueops, mcp, --http]`, EnvironmentVariables에는 `ISSUEOPS_ROOT`·`ISSUEOPS_STATE_DIR`만 있음, KeepAlive `SuccessfulExit=false`, RunAtLoad |
| `launchctl print gui/$UID/<label>` | `state = running`, `program = …/root/bin/issueops`, `pid = 56013` |
| 두 번째 `start --json` | 같은 pid 56013, `running`, 새 bootstrap 없음 |
| `server.json` / 권한 | pid 56013, started_at, executable `/private/tmp/…/issueops`, 같은 build_id. `bearer`·`server.json`·`server.lock` 0600 |
| bearer로 `initialize` | `HTTP/1.1 200`, `Issueops-Mcp-Pid: 56013`, `Issueops-Mcp-Build-Id: ab1cad68…`, `serverInfo.name=issueops` |
| bearer 없이 `initialize` | `401`, `Www-Authenticate: Bearer realm="issueops"`, identity 헤더 없음 |
| 같은 state로 foreground `mcp --http` | `another issueops mcp http instance holds the service lock (conflict)` |
| 다른 state로 foreground `mcp --http` | `127.0.0.1:47831 is unavailable (conflict): … address already in use` |
| `server.json` build_id를 바꾼 뒤 status·stop | 둘 다 `conflict`/`identity_mismatch`, exit 1. launchd pid는 여전히 56013으로 stop이 거부됐다. 원본 record는 복원했다 |
| 정상 `stop --json` | `stopped`, `launchctl print` exit 113(job 제거), `server.json` 삭제, pid 56013 종료 |
| 죽은 pid(56526)의 record를 넣은 뒤 status | `stale`/`stale_record`, pid 56526, exit 1 |
| 이어서 `start --json` | stale record를 지우고 `running` pid 56533. 새 record 기록 |
| python 프로세스가 47831을 점유한 상태에서 status·start | 둘 다 `conflict`/`port_in_use`. `launchctl print` exit 113으로 job을 로드하지 않았다 |

### 설치 dry-run

빈 HOME·state(`/tmp/jsvcmur1hqkw/dry`)에서 `issueops install --dry-run --json --path-mode=skip`(exit 0, ok, would_write 7, written 0)과 `scripts/install-native.sh --dry-run --path-mode=skip`(exit 0)을 실행했다. 두 명령 뒤에도 HOME·state 아래 파일은 0개였다. root의 `configs/`·`bin/` 해시는 바뀌지 않았고, launchd job(exit 113)과 47831 listener(0개)도 없었다.

### 설치(install-native.sh)

기존 Codex `config.toml`(model, `[mcp_servers.other]`), `~/.claude.json`(`numStartups`, `other`), `~/.omo/mcp.json`(`other`)을 0644로 만든 뒤 `scripts/install-native.sh --path-mode=skip`을 실행했다. exit 0이었고 로그 순서는 `staging` → `stopping shared MCP service before binary replacement` → `starting shared MCP service from the new binary`였다. 결과 JSON은 `ok=true`, `committed=true`, `native activation receipt sealed after strict Codex/Claude/Omo MCP and lifecycle readback`이다.

readback(bearer는 `<BEARER>`로 가렸다):

```toml
model = "gpt-test"

[mcp_servers.other]
command = "other"

[mcp_servers.issueops]
url = "http://127.0.0.1:47831/mcp"
http_headers = { Authorization = "Bearer <BEARER>" }
```

```json
{"issueops": {"headers": {"Authorization": "Bearer <BEARER>"}, "type": "http", "url": "http://127.0.0.1:47831/mcp"}, "other": {"command": "other", "type": "stdio"}}
```

Omo `mcpServers`는 `issueops={type:"http",url,headers:{Authorization:"Bearer <BEARER>"}}`, `other={command:"other"}`다. 네 host 설정(Codex·Claude·Omo·agy) 모두 0600이다. agy의 issueops entry는 `args, command, env`(stdio)를 유지했다. bearer 원문은 설정 파일 3개에만 있고, install stdout/stderr, `server.log`, plist에서는 0건이었다. 서비스는 `running` pid 57761이고 build_id `cb93fc2b…0cd7`는 교체된 `bin/issueops`의 SHA-256과 같다.

### 갱신(issueops update)

root 사본에 `cmd/issueops/jtest_update_marker.go`를 추가해 build를 바꾼 뒤 `issueops update --path-mode=skip`을 실행했다. exit 0, `ok=true`, `committed=true`였고 stop → 교체 → start 로그가 같은 순서로 나왔다. 실행 전 `running` pid 57761·build `cb93fc2b…`가 실행 후 `running` pid 58308·build `731a87c0…bd50`로 바뀌었고, 새 build는 새 `bin/issueops`의 SHA-256과 같다. `launchctl print`의 pid도 58308이었다. update 출력에 bearer는 0건이었다.

### stdio 전환과 실패 시 무변경

- `issueops install --mcp-transport=stdio --path-mode=skip`: exit 0, committed. Claude entry는 `args, command, env, type`이 됐고 Codex는 `command`/`args`/`[mcp_servers.issueops.env]` block으로 돌아왔다.
- 서비스를 stop하고 47831을 점유한 뒤 `issueops install --mcp-transport=http --path-mode=skip`: exit 1, `mcp http service is not ready (status=conflict error_code=port_in_use); host MCP configs were not changed`. 세 host 설정의 해시가 그대로였고 launchd job은 로드되지 않았다.

### 정리

마지막에 `launchctl list`의 `jtest-` 항목은 0개, 테스트 라벨 `launchctl print`는 exit 113, 47831 listener는 0개였다. 실제 `io.issueops.mcp`도 계속 로드되지 않은 상태(exit 113)다. 격리 디렉터리 `/tmp/jsvcmur1hqkw`는 증거 확인용으로 남겨 두었다.

## 알려진 한계

- launchd/systemd unit의 환경 변수는 contract대로 `ISSUEOPS_ROOT`·`ISSUEOPS_STATE_DIR`뿐이다. 따라서 서비스 프로세스의 PATH는 supervisor 기본값(launchd: `/usr/bin:/bin:/usr/sbin:/sbin`)이고, HOME은 launchd가 주는 실제 사용자 HOME이다. Homebrew의 `go`·`node`처럼 그 PATH 밖에 있는 도구를 실행하는 MCP 도구(gates check, self_verify 등)는 서비스에서 실패할 수 있다. contract에서 PATH 전달 여부를 정해야 한다.
- 응답하지 않는 lock holder(`stale`/`not_ready`)는 build_id를 확인할 수 없어서 `service stop`이 거부한다. 이때는 `launchctl bootout gui/$UID/io.issueops.mcp`로 직접 내려야 한다.
- darwin의 executable 관측은 기존 daemon inspector와 같은 `ps -o comm=`이다. 상대 경로로 띄운 foreground 서버는 다른 cwd에서 보면 executable이 달라 보여서 `conflict`로 분류된다. supervisor 경로는 절대 경로라 영향이 없다.
- linux systemd 경로는 unit 렌더링과 `systemctl show` 파싱만 단위 테스트로 검증했다. 이 장비가 darwin이라 실제 systemd 실행 증거는 없다.
- stdio로 다시 설치해도 실행 중인 HTTP 서비스는 멈추지 않는다. 필요하면 `issueops mcp service stop`을 실행한다.
- activation evidence의 semantic digest와 file SHA-256은 bearer가 들어 있는 entry·파일을 해시한다. 원문은 남지 않지만 bearer를 회전하면 receipt도 바뀐다.

## 남은 작업

- **K**
  - inspect HostIntegration에 transport=http, 서비스 `status`(pid·build_id), 실제 host receipt(discovered/connected/protocol)를 반영한다.
  - 세 host가 이 서비스(같은 pid)에 직접 연결하는지를 host별 실제 실행으로 확인한다. J는 host 연결을 주장하지 않는다. 실측에서 확인한 것은 설정 readback, 서비스 실행, 인증된 initialize까지다.
  - `internal/adapter/doctor/checks.go`의 `loopbackMCPEndpoints`는 `mcpServers`의 `type`과 `url`만 읽는다. 그래서 새 issueops http entry(`127.0.0.1:47831`)도 loopback gateway endpoint로 분류된다. 이 endpoint를 issueops 서비스로 따로 구분할지 K가 결정해야 한다.
- **L**: J 변경과 겹치는 파일은 없다. `mcp_sdk_server.go`와 catalog를 건드리지 않았다.
- **Z**
  - `TestDDDResponsibilityInventoryMatchesSource`(현재 RED)에 J의 신규 production 파일과 심볼을 등록한다. 대상 파일은 `internal/adapter/mcpservice/{instance,service,supervisor,lock_unix,lock_other}.go`, `cmd/issueops/issueopsapp/mcp_service_wiring.go`, `cmd/issueops/installcli/install_mcp_transport.go`다. 대상 심볼은 `installutil.RemoveJSONMapEntry`, claude/omo `Dependencies.RemoveJSONMapEntry`, installcli `Deps.{DefaultMCPTransport,MCPURL,MCPService}`·`MCPServiceSetup`, `update.Options.MCPTransport`, issueopsapp의 `installMCPService`·`defaultMCPTransport`·`newSupervisorMCPService`·`mcpServiceStateDir`·`mcpServiceProcessInspector`다.
  - 운영 문서(OPERATIONS/ARCHITECTURE/ADR)에 다음 내용을 반영한다: `--mcp-transport`, `mcp service start|stop|status`, LaunchAgent/systemd unit 경로, 오류 코드, `ISSUEOPS_MCP_SERVICE_LABEL`, 위 PATH 한계, update 순서, 응답 없는 서비스의 수동 bootout. root usage 문자열에도 `mcp service`와 `--mcp-transport`를 추가한다.
  - H 보고가 J 몫으로 남긴 "세션 시작 시 `issueops mcp authorize` 실행과 `authority_file` 전달" skill·hook 안내는 J의 쓰기 범위(skills·hooks 제외) 밖이라 하지 않았다. 해당 파일 owner가 이어서 처리해야 한다.
