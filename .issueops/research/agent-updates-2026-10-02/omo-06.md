# Omo MCP catalog invalidation and tool search

Retrieval date: 2026-10-02. Scope: installed implementation and public official releases; no runtime state, credentials, or transcripts inspected. Model: `gpt-6.1-sol` (`PI_MODEL`).

## Provenance and released versions

Canonical Omo project: https://github.com/code-yeongyu/oh-my-openagent, established by `/Users/habin/node_modules/omo-ai/package.json:2-24`. Installed `omo-ai` is 5.1.8, pinning `@code-yeongyu/senpi` 2026.10.1-2; the inspected runtime's `package.json:2-3` confirms that engine version.

Official notes fetched:

- https://github.com/code-yeongyu/oh-my-openagent/releases/tag/v5.1.8
- https://github.com/code-yeongyu/oh-my-openagent/releases/tag/v5.1.9

Official GitHub metadata gives publication times: v5.1.8, 2026-10-01T15:00:33Z; v5.1.9, 2026-10-02T01:13:10Z. Exact metadata URLs are `https://api.github.com/repos/code-yeongyu/oh-my-openagent/releases/tags/v5.1.8` and `https://api.github.com/repos/code-yeongyu/oh-my-openagent/releases/tags/v5.1.9`. v5.1.9 names engine 2026.10.1-3; it is released, not anticipated, but is not the inspected installation.

For anchors below, **R** means `/Users/habin/.omo/agent/runtime/a1700c8985bbd0c8-ef3ba637dc3d/dist/core/extensions/builtin/`.

## Findings

### 1. Cache validity is configuration-and-age based, not executable-content based

`R/mcp/catalog-cache.js:6-28` accepts a server entry only when its configuration hash matches and its age is at most seven days. `R/mcp/config.js:240-258` hashes stable configuration JSON with SHA-256, explicitly excluding `startupTimeoutMs`. The version-1 file is normalized and malformed reads become an empty cache (`catalog-cache.js:11-18,64-99`). Writes use a temporary file plus rename (`:43-61`).

**Certainty:** high, inspected 2026.10.1-2 implementation. **Applicability:** issueops binary replacement at an unchanged command path does not itself alter this hash. **Counterevidence:** this is not necessarily seven days of stale operation: connection refresh exists. Atomic replacement also does not prove concurrent read-modify-write updates cannot overwrite another server's update.

### 2. Cached lazy startup defers connection; live notifications refresh and tombstone

`R/mcp/service.js:342-354` uses valid cached catalogs without connecting lazy servers at startup. Cold servers and eager/keep-alive servers enter a bounded startup race; its default is 250 ms (`R/mcp/startup-race.js:7-32`). A non-shared connection refreshes and persists its catalog after connect (`:89-117`).

Tools, resources, and prompts list-change notifications are subscribed even without declared capabilities (`R/mcp/notifications.js:65-79`). Refresh scheduling defaults to 300 ms coalescing and a 1,000 ms minimum interval (`:34-61`). Removed tools receive tombstones; changed catalogs are re-registered, and connect-only unchanged registration is skipped (`R/mcp/service-tools-changed.js:35-75`).

**Certainty:** high for code paths, not measured latency. **Applicability:** distinguish cached discovery from connected readiness in issueops diagnostics. **Counterevidence:** a disconnected lazy server cannot deliver notifications; name-only deltas can say "no change" despite schema changes (`notifications.js:16-32`).

### 3. Local search is side-effect-free, but rebuilds its index per query

`R/tool-search/tool.js:4-43` returns at most five matches with schemas and source/group filters, without activation. `R/tool-search/service.js:83-89` rebuilds BM25 from the current catalog on every search. Extension discovery itself enumerates, sorts, and fingerprints documents before accepting an unchanged fingerprint (`:131-149`). BM25 weights name/group/description 3/2/1 and gates non-exact results by 0.5 term coverage and 0.35 of the best non-exact score (`R/tool-search/engine/bm25.js:1-57`).

**Certainty:** high. **Applicability:** a catalog-generation keyed index is an actionable optimization candidate; better issueops tool descriptions affect discoverability. **Counterevidence:** no benchmark establishes this as a bottleneck, and search is lexical rather than semantic. Native Anthropic search is a separate path; these findings describe local search only.

### 4. Reload correctness has a released fix, with limited independent evidence

v5.1.9 officially states that tool search keeps working after reload/session replacement. The linked report, https://github.com/code-yeongyu/senpi/issues/2509, describes a two-session reproduction on 2026.10.1-2: disposing B makes A's catalog hooks hit a stale context; its control leaves B alive. Installed `R/tool-search/service.js:211-232` confirms an AsyncLocalStorage scoped service plus a rebindable module singleton fallback.

**Certainty:** high that the release claim and implementation exist; medium for the reported reproduction, not rerun here. **Applicability:** include reload/session ownership in visibility checks, not just successful MCP connection. **Counterevidence:** the reporter's Linux/Bun reproduction is external corroboration of the failure mechanism, not independent confirmation that 2026.10.1-3 fixes every host. Release notes and installed vendor code are same-producer evidence, not independent sources.

## EXPAND

- **EXPAND-CACHE:** test unchanged-path binary replacement, lazy reconnect, and simultaneous cache writes in an isolated fixture; classify stale schemas separately from cache-file integrity.
- **EXPAND-SEARCH:** benchmark index construction and schema lookup at realistic catalog sizes before proposing generation-based caching.
- **EXPAND-RELOAD:** verify the published fix in isolated two-session and reload controls; do not infer success from version alone.

Access/failures: no inaccessible cited source. One eval JSON parse failed on bounded webfetch API text; recovered with direct structured API fetch, HTTP 200. Broad search/API output truncation was narrowed to exact code reads and release pages. No installs or production edits. Diagnostics, tests, and build are inapplicable to this evidence-only artifact; verification is citation rereading, manual scope review, and the requested scoped diff check.
