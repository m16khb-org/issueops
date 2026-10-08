---
name: install.md
description: First-run install, refresh, and compatibility install-native operations.
---

# Install And Refresh

Public setup UX has two primary commands:

```bash
# First run from a fresh clone, before issueops is on PATH.
./install.sh

# Scriptable install or refresh after issueops is on PATH.
issueops install --dry-run --json
issueops install

# Ongoing refresh from the current checkout.
io update
io inspect --json

# Initialize project docs/profile for a target repository.
issueops project bootstrap --repo /path/to/repo

# Refresh existing project docs/profile from current templates and evidence.
issueops project bootstrap --repo /path/to/repo --sync
```

`./install.sh` computes the checkout root, builds `bin/issueops` when needed, and then runs `issueops install`. In a real terminal with no arguments it enters the interactive installer. Non-interactive automation can pass explicit flags such as `--dry-run --json`.

`install` owns environment setup. Normal users should not export `ISSUEOPS_ROOT` manually; the installer writes it into Codex, Claude, Omo, and omp MCP configuration. `CODEX_HOME` is honored when already set and otherwise defaults to `~/.codex`; Omo uses its native flat-layout `~/.omo` root; omp uses `~/.omp/agent`. PATH setup is selected with `--path-mode=auto|manual|skip`. Every mode plans or writes the canonical `~/.local/bin/issueops` shim and the managed `~/.local/bin/io -> ~/.local/bin/issueops` shorthand; `manual` and `skip` only omit shell rc changes. The default `auto` mode also adds a shell rc PATH line when needed.

Each install/update refreshes user skill links for all four first-party hosts,
managed MCP registration, and the host lifecycle context surface. It also prunes
stale links in each host skill directory whose target lies under this
checkout's `skills/` but no longer exists (a removed or renamed shared skill);
links that point elsewhere or still resolve are left alone, and `--dry-run`
reports them as `would_remove` instead of deleting. Omo receives
`~/.omo/mcp.json` plus `~/.omo/extensions/issueops.js`; explicit
`--project-local` additionally writes `.omo/mcp.json`. omp receives
`~/.omp/agent/mcp.json` plus `~/.omp/agent/extensions/issueops.js`; explicit
`--project-local` additionally writes `.omp/mcp.json`.
Before any non-dry-run activation, the installer renders the complete host and
shell-path plan and snapshots every affected file, symlink, mode, and newly
created parent directory. Any host write or activation-seal failure restores
those snapshots together with the command shims before aborting the transition.

`issueops` remains the canonical command identity. `io` is a command symlink, not a shell alias or wrapper. If `~/.local/bin/io` is a regular file, directory, or points elsewhere, install/update refuses to overwrite it and requires manual resolution.

기존 `~/.local/bin/issueops`가 regular file이면 기본 install과 dry-run은 변경 없이 거부한다. 그 파일과 실제 실행 중인 staged/canonical candidate가 모두 정적 Go build identity `issueops/cmd/issueops` / module `issueops`를 만족할 때만 `--adopt-command-file`로 adoption을 명시할 수 있다. 승인된 실행은 같은 디렉터리의 mode `0600` backup을 만든 뒤 temporary symlink와 command path를 atomic exchange하고 displaced identity를 재검증한다. native activation Seal 전 오류에서는 원래 bytes와 mode를 복원하고 exact transition을 Abort한다. Seal이 성공한 뒤 backup 정리만 실패하면 activation은 committed 상태로 유지되고 JSON receipt의 `backup_retained`와 recovery path를 따른다. `io`에는 이 승인 플래그가 적용되지 않는다.

`bootstrap` and `update` use the current `issueops` checkout. They build `bin/issueops`, refresh both command shims through the same installer path, run native host installation, and refresh issueops MCP registration. They do not run `git pull`. Executable symlinks are resolved back to the checkout, so `io update` works outside the repository directory.

darwin/linux의 기본 `--mcp-transport=http` 설치는 네 host의 issueops entry를 공용 서비스
`http://127.0.0.1:47831/mcp`와 bearer 헤더로 바꾼다. 설치기는 host plan을 dry-run으로 먼저 검증하고,
서비스를 새 build로 띄운 뒤 `build_id`와 인증된 MCP 응답을 확인해야 host 설정을 merge한다.
`update`/`bootstrap`도 같은 순서로 서비스를 교체한다. 실패하면 host 설정을 바꾸지 않는다.
`--mcp-transport=stdio`는 이전 stdio entry를 설치한다. 아래 Omo·omp catalog cache token은 stdio entry에서는
`env.ISSUEOPS_MCP_CATALOG_SHA256`으로, HTTP entry에서는 `headers.X-Issueops-Mcp-Catalog-Sha256`으로
들어간다. Omo는 server config 전체(헤더 포함)를 `hashConfig`로 해싱해 catalog cache 키로 쓰므로,
catalog가 바뀌면 두 transport 모두 다음 세션이 새 `tools/list`를 조회한다. 서버는 이 헤더를 읽지 않는다
(`internal/adapter/omo/mcp.go`의 `omoMCPCatalogHeader`, `TestOmoHTTPEntryChangesWithTheAdvertisedCatalog`).

Linux에서 HOME을 임시 디렉터리로 바꿔 설치하면(CI 등) HTTP 설치가 실패한다. installer는 unit을 그 HOME의
`~/.config/systemd/user/`에 쓰지만, 이미 실행 중인 user systemd manager는 자기 HOME 기준 경로에서 unit을 찾기 때문이다.
그래서 HTTP 설치는 credential·log·unit을 쓰기 전에 `systemctl --user show-environment`로 manager의 `HOME`과
`XDG_CONFIG_HOME`을 읽고, unit 디렉터리가 manager의 기대 디렉터리와도 `systemctl --user show -p UnitPath --value`
목록과도 맞지 않으면 아무것도 쓰지 않고 `unit directory ... is not searched by the running systemd user manager`
오류로 멈춘다. 조회가 실패하거나 HOME이 없거나 값이 `$'...'`로 이스케이프돼 판정할 수 없으면 막지 않고, 기존 load 단계가
실패한 명령과 출력을 담아 실패한다. 이런 환경에서는 manager와 같은 HOME에서 설치하거나 `--mcp-transport=stdio`를 명시한다.
supervisor를 쓸 수 없어도 installer가 stdio로 자동 전환하지 않는 것은 의도된 동작이다
([ADR 2026-10-02](../adr/2026-10-02-shared-streamable-http-mcp-and-caller-capability.md)). CI의 self-verify 단계는 stdio를
쓰고, 별도 단계가 임시 HOME HTTP 설치가 쓰기 전에 이 오류로 멈추는지 확인한다.

stdio `issueops mcp`는 host 세션 안에서 in-process로 실행된다. `io update`는 host가 소유한 stdio MCP 프로세스를 열거하거나 종료하지 않으므로, 새 binary의 MCP 동작은 host에서 서버를 재연결(reconnect)할 때 적용된다. 실행 모드의 정규 설명은 [runtime 문서](../architecture/runtime.md)를 따른다.

Omo는 MCP tool catalog를 server config hash 기준으로 최대 7일 재사용하므로, 같은
경로의 binary만 교체하면 새 세션도 이전 input schema를 유지할 수 있다. Omo
installer는 현재 advertised tool catalog의 SHA-256을 stdio entry의
`ISSUEOPS_MCP_CATALOG_SHA256` env(HTTP entry에서는 `X-Issueops-Mcp-Catalog-Sha256` 헤더)에 기록한다. 따라서 `install`/`bootstrap`/`update`
후 catalog가 바뀌면 Omo server config hash도 바뀌고, 다음 세션은 새
`tools/list`를 조회한다. 이 값은 cache revision token이며 MCP handler 동작을
제어하지 않는다. omp도 MCP tool 목록을 config hash로 cache하므로 omp installer가
`~/.omp/agent/mcp.json`의 issueops entry에 같은 token을 기록한다.

외부 GitLab MCP와 개인 wrapper 등록은 update에 포함되지 않는다. 필요할 때만 `scripts/sync-glab-mcp.sh --dry-run`으로 확인한 뒤 `scripts/sync-glab-mcp.sh`를 명시적으로 실행한다.

Default user-level install updates:

- Command shims: `~/.local/bin/issueops -> <issueops>/bin/issueops`, `~/.local/bin/io -> ~/.local/bin/issueops`
- Codex skill symlinks: `~/.codex/skills/* -> <issueops>/skills/*`
- Claude skill symlinks: `~/.claude/skills/* -> <issueops>/skills/*`
- Codex MCP config: `~/.codex/config.toml` `[mcp_servers.issueops]`
- Codex hooks: `~/.codex/hooks.json`
- Claude hooks: `~/.claude/settings.json`
- Claude user-scope MCP config: `~/.claude.json` key `mcpServers.issueops` (written directly by the installer)
- Omo skill symlinks: `~/.omo/agent/skills/* -> <issueops>/skills/*`
- Omo MCP config: `~/.omo/mcp.json`
- Omo lifecycle extension: `~/.omo/extensions/issueops.js`
- omp skill symlinks: `~/.omp/agent/skills/* -> <issueops>/skills/*`
- omp MCP config: `~/.omp/agent/mcp.json` key `mcpServers.issueops` (mode `0600`)
- omp lifecycle extension: `~/.omp/agent/extensions/issueops.js` (`session_start`/`session_switch` -> `hook session-start`, `session_compact` -> `hook post-compact`, no accepted filter; exports `ISSUEOPS_OMP_SESSION_ID` from the main session)
- Optional Claude Code plugins and Git skills declared in `configs/upstream.json` (currently Claude-scoped): entries already present are skipped, and upstream failures are reported as `upstream ...` messages without failing native installation. See [hosts.md](guides/hosts.md#upstream-plugins-and-skills).

Default install does not create target-repo `.claude/settings.json`,
`.mcp.json`, `.omo/mcp.json`, `.omp/mcp.json`, or `.agents/mcp_config.json`. Use explicit
project-local options only when a repo should own those MCP files. Repo-local
skill links (`.claude/skills`, `.omo/skills`, `.agents/skills`) are never
created: user-scope links already resolve to the same `skills/` source.

Dry-run checks:

```bash
./install.sh --dry-run --json
issueops install --dry-run --json
issueops bootstrap --dry-run --json
```

Release reproducibility smoke:

```bash
scripts/release-repro-smoke.sh
```

This script builds the current checkout, then verifies `install --dry-run --project-local --json` in temporary `HOME`, `CODEX_HOME`, and fixture `ISSUEOPS_ROOT` directories. It also checks the clean `inspect/docs/state` workflow under a temporary state directory.

Release build matrix smoke:

```bash
scripts/release-build-matrix.sh
```

The default release matrix cross-builds `darwin/arm64`, `darwin/amd64`, `linux/amd64`, and `linux/arm64` with `CGO_ENABLED=0`.

## Project `--sync`

`issueops project bootstrap --sync` refreshes target repo `AGENTS.md` routing block, `.issueops/*.md`, and user-state repo profile metadata from current evidence.

Use low-level `scripts/install-native.sh` and `install-native` directly only for automation or focused installer debugging.
