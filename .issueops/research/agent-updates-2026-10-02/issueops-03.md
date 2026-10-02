# Host adapter installation, updates, and capability detection

Retrieval date: 2026-10-02. Scope: local source and neighboring tests, plus official public release records. Tests were inspected, not executed; no installation, benchmark, configuration mutation, or production edit occurred. Local observations describe this checkout, not a released issueops version. External statements below are official single-source claims; release notes, changelogs, and release metadata from one publisher are not independent corroboration.

## Findings

### 1. Updates already have staging and activation boundaries

**High certainty; directly applicable.** `scripts/install-native.sh:160-248` builds a staged executable, runs its version command and installation dry-run, begins activation, fsyncs and replaces the binary, then checks a seal receipt for the matching transition and `committed=true`. `internal/application/update/service.go:23-50` forwards options, stops on installer failure, and skips daemon refresh on dry-run. Neighboring `internal/application/update/service_test.go:25-48` explicitly covers ordering, dry-run, and installation failure.

**Opportunity:** expose stage durations and activation status before proposing incremental-build or repeated-preflight optimization. **Counterevidence:** staging, durability, and activation checks have correctness purposes; source inspection alone does not establish redundant work or a bottleneck. `--skip-build` already supplies an explicit rebuild bypass. No speedup was measured.

### 2. Preflight readiness is narrower than host capability

**High certainty; directly applicable to visibility.** `internal/adapter/hostprobe/runner.go:189-221` sets `Installed` after executable lookup and `Ready` after successful `--version`; it does not compare versions or demonstrate MCP/hook support. Version output is redacted, limited to its first line, and bounded to 256 bytes. The version timeout is ten seconds (`runner.go:24-29`). `internal/adapter/hostprobe/codex_test.go:34-72` asserts these exact preflight outcomes.

**Opportunity:** distinguish executable/version readiness from observed integration capability in reports. **Counterevidence:** this is not proof that issueops lacks behavioral verification: `internal/adapter/hostprobe/codex.go:86-115` executes an isolated episode and requires valid runtime and SessionStart observations. Preserve that distinction rather than turning every installation into a live model call.

### 3. Build-generation warnings cannot uniquely identify dirty builds

**High certainty; directly applicable to update diagnostics.** `internal/adapter/install/native_generation.go:12-40,61-72` derives identity from VCS revision and a modified boolean, displaying twelve revision characters plus `+dirty`. Two differently modified binaries built at the same revision therefore receive the same display identity. Missing build metadata becomes unobserved rather than an error (`native_generation.go:42-59,76-93`). `native_generation_test.go:9-44` covers display values and unreadable paths.

**Opportunity:** assess whether existing activation binary digests should appear alongside generation warnings. **Counterevidence:** generation strings are diagnostics, not necessarily the activation authority; finding 1 already shows SHA-256-bound activation receipts. Do not replace the safety contract or claim a demonstrated activation vulnerability.

### 4. Codex hook installation is static and deliberately minimal

**High certainty for local behavior; official single-source release dating.** `internal/adapter/codex/install.go:17-33` writes configuration without a host-version probe. `install_hooks.go:74-82` installs only a five-second SessionStart command; its comment cites behavior verified against Codex CLI 0.150.1. `install_test.go:19-94` asserts context-only hooks and idempotent installation; lines 96-112 preserve a co-resident hook.

Codex 0.150.1 was published **2026-08-27**, not a future release: https://api.github.com/repos/openai/codex/releases/tags/rust-v0.150.1. Its official notes, https://github.com/openai/codex/releases/tag/rust-v0.150.1, describe retained-image compaction budgeting, not the hook schema.

**Opportunity:** document the tested host version separately from installed-version evidence. **Counterevidence:** release notes do not independently confirm the comment's hook claim; this lane did not rerun the host. Minimal hooks already avoid installing the removed per-tool surfaces, but their performance benefit is unmeasured.

### 5. Claude's new tool-deferral switch is a compatibility lead

**High certainty that the publisher reports it; medium applicability.** Claude Code **2.1.287**, published **2026-10-01**, changes MCP `alwaysLoad: false` to defer all server tools behind tool search. Sources: https://github.com/anthropics/claude-code/releases/tag/v2.1.287 and https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.287.

`internal/adapter/claude/install_mcp.go:5-25` emits a stdio server with command, arguments, and environment, without `alwaysLoad`. `install_test.go:12-49,72-88` checks user-scope defaults and explicit project-local configuration.

**Opportunity:** evaluate opt-in deferral against discoverability and actual tool use. **Counterevidence:** these settings do not prove tools are currently loaded eagerly or that deferral improves issueops; no baseline, live compatibility test, or independent corroboration was obtained. Omo similarly writes its own MCP and extension files (`internal/adapter/omo/install.go:25-76`); Claude's option cannot be assumed portable.

## EXPAND

- **EXPAND-CAPABILITY:** connect version readiness, activation evidence, and observed hook/MCP capability without conflating them.
- **EXPAND-IDENTITY:** inspect digest availability at warning sites before adding another identifier.
- **EXPAND-DEFERRAL:** verify Claude 2.1.287 opt-in discovery behavior before measuring latency/context effects.
- **EXPAND-PERFORMANCE:** measure update stages before removing checks or adding caches.

Access failures: none for cited public URLs. Failed probes: unavailable `glob`/`grep` tool calls; a source search included nonexistent `internal/adapter/bootstrap` and reported that error, then was narrowed. No failed install/test/build commands occurred because none ran.

Verification: all four cited public URLs were reopened through webfetch; key local source ranges were reread. `git diff --check -- .issueops/research/agent-updates-2026-10-02/issueops-03.md` returned exit 0. This new, untracked report is not represented by ordinary Git diff, so its content and word count were also checked directly. Diagnostics, tests, builds, and benchmarks were not run for this bounded evidence-only report.
