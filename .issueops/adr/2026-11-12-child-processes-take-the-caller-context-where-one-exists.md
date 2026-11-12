---
name: 2026-11-12-child-processes-take-the-caller-context-where-one-exists
description: Accepted decision record with rationale, alternatives, and consequences.
---

# 자식 프로세스는 호출자 context가 있는 경로에서만 그 context를 받는다

- Date: 2026-11-12
- Kind: `adr`
- Source: claude-code main session (round 3, WS-C `ws-execctx`)
- Summary: 호출자 context가 있는 경로에서 시작하는 자식 프로세스는 그 context를 `exec.CommandContext`로 받는다. 호출자 context가 없는 경로는 CLI가 root context를 갖기 전까지 예외이며, 남은 `exec.Command` 17곳을 사유 분류와 함께 architecture ratchet allowlist에 묶는다. 이 결정은 [2026-11-04 결정](2026-11-04-effective-go-context-first-provider-port-stdio-mcp.md)의 Summary와 색인 항목에 있는 일반 규칙 "외부 명령·HTTP 호출은 context를 첫 인자로 받는다"를 자식 프로세스에 대해 좁힌다(supersedes in part): 호출자 context가 있는 경로에서는 그대로 지키고, 없는 경로는 목록에 오른 예외다. 같은 기록의 결정 3)(`port.IssueProvider` 메서드의 ctx-first 전환과 `providerutil.RunContext`)은 바뀌지 않는다.
- Context: 2026-11-04 기록의 Summary와 `conventions/go-and-packages.md`는 외부 명령·HTTP 호출이 `context.Context`를 첫 인자로 받는다고 적었지만, 같은 문서의 git 실행 규칙은 `exec.Command(gitbin.Path(), ...)`를 예로 들었다. 그보다 앞선 `.issueops/issues/572/architecture.md:125`는 "ctx가 없는 로컬 git 실행까지 바꾸려면 호출 경로 전체의 시그니처가 바뀐다. 취소가 의미 있는 네트워크 호출만 바꾼다"며 일괄 전환을 기각했으나 ADR로 남지 않았다. 2026-11-12 조사에서 production `exec.Command`는 19곳이었다. MCP handler는 요청 context를 갖지만(HTTP는 요청 context, stdio는 종료 때만 취소) CLI에는 root context가 없어 호출 경로가 `context.Background()`에서 끝난다. `lint_diagnose`는 호출자가 준 argv를 시간 제한 없이 실행했다.
- Decision: 1) 규칙: 호출자 context가 있는 경로에서 시작하는 자식 프로세스는 그 context를 받는다. 호출자 context가 없는 경로는 CLI root context가 생길 때까지 목록에 오른 예외다. 2) 이번에 바꾼 곳은 둘이다. `commitsuggest.Effects.Diff`(MCP `commit_suggest`)는 `exec.CommandContext`로 git을 실행하고, 죽어도 `.git/index.lock`을 남기지 않도록 `--no-optional-locks`를 붙인다. `lintdiagnose.Effects.Run`(MCP `lint_diagnose`)은 호출자 context에 15분 상한(`lintdiagnose.Service.CommandTimeout`, 0이면 15분)을 씌운 context로 실행한다. 이 상한은 lint 예산이 아니라 멈춘 명령을 끊는 안전장치다(`lint_diagnose`는 비어 있지 않은 argv를 모두 받고 스킬은 테스트 명령도 이 도구로 보낸다). 끊기면 `Diagnose`는 context 에러를 감싼 에러를 돌려주고 부분 출력으로 진단을 만들지 않는다. 상한에 걸리면 `lint command timed out after 15m0s`, 호출자 context가 먼저 끝나면(취소 또는 호출자 자신의 deadline) `lint command stopped by the caller`다. 두 곳 모두 `cmd.WaitDelay`(2초)를 둔다. Go는 이 타이머를 context가 끝날 때와 프로세스가 끝날 때 중 먼저 오는 쪽에서 시작한다. 시작한 프로세스가 만든 다른 프로세스가 pipe를 잡고 있어도(Windows에서 죽인 launcher의 자식, 백그라운드로 남은 자식) 출력 읽기가 2초 뒤 끝난다. 프로세스가 스스로 끝났는데 pipe만 남아 `exec.ErrWaitDelay`가 나면 그 프로세스의 종료 코드를 그대로 쓴다(`Run`은 그 코드와 `failed = code != 0`, `Diff`는 git이 0으로 끝났으면 출력과 nil 에러). 시작한 프로세스만 죽이며 프로세스 트리 전체 종료는 범위 밖이다. CLI 진입점은 `context.Background()`를 넘긴다. 3) `internal/architecture/exec_command_test.go`가 build tag와 상관없이 `cmd`와 `internal`의 production `.go` 파일을 AST로 읽어 선언별 `exec.Command` 수를 allowlist(17항목, 합계 17)와 맞춘다. 새 호출, 개수 차이, 남은 항목은 실패다. 항목마다 분류(`own-lifecycle`, `installer`, `no-caller-context`, `fail-open-on-cancel`, `wide-signature-change`)와 설명이 있어야 한다. allowlist 없이 막는 형태: `os/exec` dot import, `exec.Cmd{}`·`&exec.Cmd{}`·`new(exec.Cmd)`, 포인터가 아닌 `exec.Cmd` 변수와 struct field, `os.StartProcess`, `syscall.StartProcess`·`ForkExec`·`Exec`, 첫 인자가 `context.Background()`나 `context.TODO()` 호출식인 `exec.CommandContext`. 한 파일이 `os/exec`를 여러 이름으로 import해도 모두 센다. 판정은 구문만 보므로 변수를 거친 root context(`ctx := context.Background(); exec.CommandContext(ctx, ...)`)는 잡지 못한다. 4) 공용 helper 패키지는 만들지 않는다. 각 변경이 한 줄이고 dependency 규칙을 바꿀 이유가 없다.
- Consequences: MCP `commit_suggest`와 `lint_diagnose`는 요청이 취소되면 자식 프로세스를 멈추고 기존 실패 payload(`commit_suggest failed`, `lint_diagnose failed`)로 끝난다. 성공 응답 모양과 response-contract golden은 바뀌지 않는다. `issueops project lint-diagnose`도 15분 상한을 받는다. 백그라운드 자식을 남기고 끝나는 명령은 전에는 그 자식이 끝날 때까지 기다렸지만, 이제 종료 2초 뒤 그때까지의 출력으로 끝난다. `.issueops/issues/572/architecture.md:125`의 판단은 유지된다(로컬 CLI git은 바꾸지 않는다). 다만 MCP에서 닿고 취소가 안전한 경로는 이번처럼 바꾼다. `AGENTS.md`의 "shell 실행 기능은 allowlist/denylist, timeout, cwd, env redaction, audit log를 포함해야 한다"는 `lint_diagnose`에서 아직 다 지켜지지 않는다. 이번에 timeout과 호출자 취소만 생겼고 allowlist, env redaction, audit log는 남은 과제다.
- Held follow-ups:
  - CLI root context(신호에 묶인 context를 진입점에서 만들어 아래로 넘긴다). 이것이 먼저다.
  - 그 뒤 `no-caller-context`와 `fail-open-on-cancel` 사이트: `augmentation`, `docs`, `guard`, `issueops/material_files.go`, `gitraw`, `issueopsinventory`, `update/runtime.go`, `verifywork/git.go`.
  - `processinspect`: `issueops update`뿐 아니라 MCP 서비스 supervisor(`cmd/issueops/issueopsapp/mcp_service_wiring.go`의 `mcpServiceProcessInspector`)도 쓴다. port가 `func(int)`라 context가 없으므로 port에 ctx를 더한 뒤에 바꾼다.
  - `gitexclude.EnsureWorkspaceState`: 두 호출자(`issueops/execution_lease_storage.go`, `issueopslease.ReseedArtifacts.Prepare`)가 에러를 버린다(`_ = gitexclude.EnsureWorkspaceState(...)`). 취소되면 exclude 줄을 조용히 건너뛰므로 호출자가 에러를 다루게 한 뒤에 바꾼다.
  - `quality/coverage.go`의 `gitCoverageFingerprintInput`: 호출자 context가 없다(`RunGoTestCoverage`는 10분 context를 `context.Background()`에서 만든다). 실패하면 cache를 건너뛸 뿐이다.
  - `policy.CommandExecutor.Execute`: `gates`, `verifywork`, `worker`, policy CLI가 함께 쓰고 자체 timeout과 process-group 감독이 있어 영원히 멈추지 않는다. 부모 context를 넘기려면 `supervisedExecution`에서 timeout과 취소를 구분하는 의미부터 설계해야 한다.
- Evidence:
  - 조사: production `exec.Command` 19곳 → 이 결정 뒤 17곳(`commit_suggest.go`, `lintdiagnose/diagnose.go`가 빠짐)
  - `go test ./internal/architecture -run ExecCommand -count=1 -v`
  - `go test ./internal/adapter/commitsuggest ./internal/adapter/lintdiagnose ./internal/application/lintdiagnose -count=1` (취소 시 5초 안 반환, 200 ms 상한의 timeout, 부분 출력 미사용, 백그라운드 자식이 stdout을 잡아도 종료 코드 유지)
  - `go test ./cmd/issueops/mcpcli -run CallerContext -count=1 -v` (MCP 요청 context가 두 effect까지 닿는다)
  - `exec.Command`로 되돌리면 두 취소 테스트가 5초 timer로 실패하고, Windows에서 `WaitDelay`를 빼도 같은 timer로 실패한다. `ErrWaitDelay` 처리가 없으면 0으로 끝난 명령이 실패(`-1`)로, 0으로 끝난 git diff가 에러로 보고된다
- Alternatives / rejected options:
  - 모든 사이트를 한 번에 `exec.CommandContext`로 전환: CLI에 root context가 없어 대부분 `context.Background()`를 넘기게 되고, `gitraw.Run`은 함수 타입 선언 21개(14개 파일)를 바꿔야 하며, 쓰기 중인 git을 죽이면 저장소 상태를 반쯤 남길 수 있다
  - context가 없는 사이트에 `context.Background()` 기반 deadline 추가: fail-open 사이트에서는 시간 초과가 조용한 오판정이 된다
  - `gitexclude`, coverage fingerprint, policy executor도 이번에 전환: 위 held follow-up의 이유로 리뷰에서 빠졌다
  - 공용 실행 helper 패키지: 바꾸는 곳이 두 줄이라 abstraction 근거(사용처 둘 이상의 공통 계약)가 없다
