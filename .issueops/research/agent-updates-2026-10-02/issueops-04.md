# SessionStart injection and project-document loading

Retrieval date: **2026-10-02**. Scope: repository source, neighboring tests, and public official documentation; no runtime benchmarks. Local anchors describe the current checkout, not a released issueops version. Tests were inspected, not executed. Host documentation and vendor release records are official single-source evidence; they are not independent corroboration. Repository tests corroborate intended local behavior, not host delivery or performance.

## Findings

### 1. A metadata-only prompt still requires document-body I/O

**Certainty: high for implementation; optimization benefit unmeasured.** `cmd/issueops/issueopsapp/hook_facade.go:12-24` wires SessionStart catalog discovery directly to `DiscoverProjectDocs`. Discovery opens top-level `.issueops/*.md`, reads each eligible file, parses frontmatter, and extracts its first H1 before formatting only descriptions/titles into the menu (`internal/adapter/projectdoc/catalog.go:39-89,100-145,150-161`). There is no cache in this call path. `internal/application/hookprompt/catalog_test.go:26-44` checks metadata and both rendered views.

**Applicability:** a bounded header reader could reduce filesystem and parsing work without injecting full documents. **Counterevidence:** H1 fallback may occur beyond frontmatter, custom descriptions must remain supported, and current reads already have limits. This is a candidate to measure, not evidence of a slow SessionStart.

### 2. Existing bounds hide omissions and do not ensure stable membership

**Certainty: high for code structure; cross-filesystem membership variation is an inference.** Limits are 64 accepted documents, 128 raw directory entries, 256 KiB per accepted file, and 2 MiB aggregate accepted content (`internal/adapter/projectdoc/catalog.go:12-16,64-88,92-122`). The raw `File.ReadDir(128)` batch is selected before the final path sort. Consequently, sorting stabilizes returned order, not membership when caps exclude entries. Rejected oversized reads do not increment the accepted-byte counter, so 2 MiB is not an absolute total-I/O ceiling. Skips and directory failures return no diagnostic reasons.

**Applicability:** expose omission counts/reasons and define deterministic capped selection before pursuing caching. **Counterevidence:** safety exclusions for symlinks/nonregular files are intentional; boundedness tests exist (`internal/adapter/projectdoc/catalog_test.go:85-130`). Those tests do not assert a stable selected subset or diagnostics.

### 3. Hosts receive different catalog presentations

**Certainty: high for emitted payloads, not independently verified native rendering.** Claude receives compact `additionalContext` plus readable `systemMessage`; Codex receives the readable view as `additionalContext`, without `systemMessage` (`internal/adapter/hostprotocol/hook.go:5-19`; assertions: `cmd/issueops/hookcli/hook_catalog_test.go:67-79,126-136`). Omo sends `payload.compact` with `display:false` and `triggerTurn:false` (`configs/omo/issueops.js:4-25`). Catalog building renders both views even when a host consumes only one (`internal/application/hookprompt/catalog.go:16-24`).

**Applicability:** measure model-facing bytes/tokens separately from visible output and rendering work. **Counterevidence:** readable Codex context is explicitly tested and may serve a visibility requirement; smaller formatting is not automatically better. The local hook comment records Claude 2.1.247/Codex 0.150.1 verification dated 2026-08-27 (`cmd/issueops/hookcli/hookcatalog/catalog.go:20-25`), but that historical comment is not a fresh host execution.

### 4. Routing, reading, and native instructions are separate cost surfaces

**Certainty: high for local behavior; native discovery is official single-source.** Routing returns paths, reasons, and existence, adding existing family overviews without reading their contents (`internal/application/projectdocs/route.go:17-40`; test: `internal/application/projectdocs/route_test.go:16-31`). On-demand reads use whole-file `os.ReadFile`, return full content, and hash it (`internal/adapter/projectdocs/project_docs_revise_effects.go:11-19`; `internal/application/projectdocs/read.go:15-31`). Missing-versus-empty semantics are tested in `internal/application/projectdocs/read_test.go:18-28`.

Official Codex guidance says AGENTS.md discovery happens once per run and defaults to a combined 32 KiB limit: https://developers.openai.com/codex/guides/agents-md (living documentation; no release date/version supplied).

**Applicability:** account separately for native instructions, injected menus, and subsequent document reads. **Counterevidence:** that 32 KiB limit concerns native instruction discovery, not issueops hook output or tool-returned documents; routing alone does not demonstrate lower token consumption.

### 5. Claude offers relevant visibility, but startup context remains latency-sensitive

**Certainty: official single-source, high for documented availability.** https://code.claude.com/docs/en/hooks#sessionstart says startup hooks can run behind the interactive UI, yet the first response waits for completion. Resume/fork inputs can expose `context_tokens`, cache-expiry likelihood, elapsed staleness, and estimated cache-write cost starting in **2.1.251**, published **2026-08-28**: https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.251. Documentation and release notes share a vendor and are not independent evidence.

**Applicability:** optional host-side diagnostics could distinguish resumed-context cost from catalog cost. **Counterevidence:** these estimates cover resumed context, not hook duration, and fields are conditional. Current issueops smoke observation records only event/model and requires explicit opt-in (`cmd/issueops/hookcli/hookcatalog/live_probe_observation.go:14-38`). No production telemetry addition is justified by this lane alone.

## EXPAND

- **EXPAND-MEASURE:** separately measure discovery bytes, parsing time, emitted payload size, and first-response delay before claiming improvement.
- **EXPAND-CORRECTNESS:** test stable capped membership, skipped-file visibility, and description-size limits; preserve fallback and filesystem safety.
- **EXPAND-VISIBILITY:** assess optional resume-cost diagnostics across supported versions without turning context hooks into state-maintenance hooks.

Verification: key URLs were reopened and cited code ranges reread. The scoped `git diff --check -- .issueops/research/agent-updates-2026-10-02/issueops-04.md` returned exit 0. No tests, builds, language-server diagnostics, installs, or benchmarks ran, as this lane is source-only. No inaccessible public sources or failed shell commands. Discovery encountered an ENOENT read for `internal/domain/projectdoc/catalog.go` and a missing grep path `internal/adapter/outbound/projectdocs`; actual catalog/reader paths above resolved both. Large public responses were truncated by webfetch; full hook sections and release metadata were additionally retrieved through read-only HTTP.
