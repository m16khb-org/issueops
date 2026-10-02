# Skills loading parity and documentation footprint

Retrieval date: **2026-10-02**. Scope: read-only repository inspection and public official sources; only this report is written. Model: `gpt-6.1-sol` (`PI_MODEL`). Findings describe inspected code and test assertions, not executed host behavior or measured performance.

## Source and release boundaries

Official rolling documentation: [OpenAI skills](https://developers.openai.com/codex/skills), [Claude Code skills](https://code.claude.com/docs/en/skills), and [Agent Skills specification](https://agentskills.io/specification). These pages do not establish a release date or minimum version for every claim below. The official OmO [release API](https://api.github.com/repos/code-yeongyu/oh-my-openagent/releases/latest) returned **v5.1.9**, published **2026-10-02T01:13:10Z**, with `prerelease=false`. This establishes a real release, not its installation here or discovery semantics. Repository documentation records historical checks against Codex **0.142.0** on **2026-06-23** and **0.150.1** (`.issueops/operations/hosts.md:29-46`); these are local observations, not independently verified release dates.

Vendor-specific claims remain official single-source evidence. OpenAI and Anthropic independently document progressive loading in their respective products, corroborating the general pattern, not identical implementation. Repository tests corroborate intended issueops contracts, not independent real-host execution.

## Findings

### 1. Shared source links do not establish current discovery parity

**High certainty for code; medium for compatibility risk.** Installers link into `<CodexHome>/skills`, `~/.claude/skills`, and `~/.omo/agent/skills` (`internal/adapter/codex/install.go:15-21`, `internal/adapter/claude/install.go:16-22`, `internal/adapter/omo/install.go:20-31`). The matrix asserts shared targets, host-specific exclusions, and no repository-local skill links even with project-local MCP enabled (`internal/adapter/install_contract_matrix_test.go:657-709`).

OpenAI's current page instead documents user discovery at `~/.agents/skills` and explicitly supports symlinked folders. Claude's page documents `~/.claude/skills` and symlink-target deduplication. **Applicability:** verify Codex's legacy directory support before changing installation paths. **Counterevidence:** historical issueops Codex checks used `.codex/skills`; absence from today's documented table does not prove that path stopped working. The matrix checks filesystem layout, not native skill selectors.

### 2. Documentation structure supports selective loading, but costs differ

**High certainty for structure and documented policies.** The issueops router maps ten stages to separate skills and explicitly avoids reading issue and PR skills together (`skills/issueops/SKILL.md:62-85`). OpenAI documents an initial skills-list budget of **2% of context**, or **8,000 characters** when context size is unknown, with description shortening and possible omission. Claude documents body-on-use loading and a **1,536-character** combined description/`when_to_use` cap. The specification recommends bodies below **5,000 tokens** and **500 lines**.

**Applicability:** front-load triggers and keep stage procedures in their owning skills. **Counterevidence:** splitting bodies does not eliminate the installed-description catalog; more skills can increase discovery footprint. No token, latency, or omission measurement was performed, so savings remain hypotheses.

### 3. Inspect exposes a narrower picture than installation

**High certainty.** `Inspect` checks a selected Codex skill under hardcoded `~/.codex`, Claude user/project skills, and MCP presence, but has no OmO integration check in this constructor (`internal/adapter/inspect/inspect.go:14-38`). `ListSkills` accepts `skillName` without using it and reads every directory's entire `SKILL.md` to extract a single-line description (`internal/adapter/inspect/inspect.go:41-78`).

**Applicability:** distinguish source inventory, link validity, and native discovery in visibility work; investigate metadata-only reads if profiling justifies them. **Counterevidence:** this is an inspection path, not proof of startup overhead. The neighboring test checks inventory, identity, and project MCP detection (`internal/adapter/inspect/inspect_test.go:12-33`), not three-host discovery.

### 4. Local validation differs from the current portable specification

**High certainty.** The validator allows five fields, excluding standard `compatibility` (`scripts/validate-skill.py:15-16`); it rejects unexpected fields but checks descriptions only for type and non-emptiness (`scripts/validate-skill.py:98-142`). The specification allows `compatibility` and limits descriptions to **1,024 characters**. Neighboring tests cover quoted descriptions, unknown keys, and bad names (`scripts/validate_skill_test.py:28-65`).

**Applicability:** add focused compatibility and description-boundary cases in a future implementation lane. **Counterevidence:** a deliberately narrower authoring policy can be valid; these differences alone do not prove any shipped skill fails a host.

### 5. Host filtering intentionally fails open

**High certainty.** Missing, unreadable, malformed, or empty `install.json` enables every host (`internal/adapter/installutil/skill_hosts.go:12-43`). Tests explicitly preserve malformed-config behavior (`internal/adapter/installutil/skill_hosts_test.go:38-53`); skipped-skill messages occur only after successful partitioning (`internal/adapter/installutil/install_util.go:14-32`).

**Applicability:** a warning could make accidental cross-host exposure visible. **Counterevidence:** changing the default would alter a tested compatibility contract; invalid metadata is not currently a policy failure.

## EXPAND

- **EXPAND-DISCOVERY / compatibility:** verify legacy Codex discovery and OmO v5.1.9 native discovery using pinned official implementation or a separately authorized host probe.
- **EXPAND-VISIBILITY / correctness:** design host-aware inventory and link-target reporting; retain contract coverage.
- **EXPAND-METADATA / correctness:** reconcile portable validator limits and expose malformed host filters without silently changing defaults.
- **EXPAND-FOOTPRINT / measurement:** measure catalog/body tokens and inspection reads before claiming optimization benefits.

Verification: key public URLs were reopened and cited code/test ranges read. Inaccessible sources: none. Required `git diff --check -- .issueops/research/agent-updates-2026-10-02/issueops-10.md` exited 0. Additional nonzero command: `git diff --no-index --check -- /dev/null .issueops/research/agent-updates-2026-10-02/issueops-10.md` exited 1 with no whitespace diagnostics (new-file differences); it is not counted as a passing gate. Broad tests, benchmarks, builds, installs, and state mutations were excluded by scope.
