# Aider context-map caching and edit verification

Retrieved: 2026-10-02
Scope: Public Aider sources and IssueOps repository; not a benchmark or implementation proposal. Aider docs/release notes are single-source claims; no independent corroboration was established.

## Release snapshot

The latest verified official release is **Aider v0.86.1**, published **2025-08-13** (PyPI metadata; version-bump commit same day). It predates the 90-day window, so evidence widens to current docs and release history. The history's undated “main branch” is not treated as a release. Historical v0.51.0/v0.59.0 entries lack inline dates.

Sources: [PyPI v0.86.1 metadata](https://pypi.org/pypi/aider-chat/0.86.1/json), [version-bump commit](https://github.com/Aider-AI/aider/commit/b8b521f143f06be420e0c3ea4fe0a7fddc8db0da), [Aider release history](https://aider.chat/HISTORY.html).

## Findings

### 1. The repository map is a ranked, budgeted summary, not a cached copy of the codebase

Aider's map lists files and key definitions/signatures, ranking a dependency graph to fit relevant symbols within a token budget. `--map-tokens` defaults to 1k and may grow when no files are in chat. This is context selection, not proof of avoided scans or source-content caching. **v0.51.0** notes less frequent map recomputation in large/mono repos or with prompt caching and adds `--map-refresh`; **v0.59.0** reports deterministic maps and improved caching. No benchmark is given. **Certainty:** High for documented behavior; Aider single-source. **Applicability:** measure context construction and emitted bytes/tokens separately from provider cache hits; prefer bounded context, consistent with IssueOps MCP guidance (`.issueops/architecture/runtime.md`, “MCP tool design guidance”). **Counterevidence:** Aider's code-symbol map is unlike IssueOps's markdown docs index (`.issueops/architecture/runtime.md`, “Docs / state / config / logs”).

Sources: [Repo map docs](https://aider.chat/docs/repomap.html), [configuration options: `--map-tokens`, `--map-refresh`](https://aider.chat/docs/config/options.html#repomap-settings), [HISTORY: v0.51.0](https://aider.chat/HISTORY.html#aider-v0510), [HISTORY: v0.59.0](https://aider.chat/HISTORY.html#aider-v0590).

### 2. Prompt caching and map-refresh caching are distinct controls

`--cache-prompts` is opt-in (default false); the guide lists provider/model-specific support, including Anthropic and DeepSeek. Aider arranges reusable prefixes from system prompt, read-only files, repo map, and editable files. Map refresh is a separate `auto`/`always`/`files`/`manual` policy (default `auto`). Optional keepalive pings address the documented five-minute Anthropic cache expiry. Streaming hides cache statistics/costs. **Certainty:** High for documented behavior; official single-source, not independently validated provider internals. **Applicability:** distinguish recomputation, prompt-prefix reuse, cache metrics, and elapsed time. **Counterevidence:** no latency/savings figures; streaming limits visibility. **v0.51.0** introduced prompt caching/map refresh; **v0.53.0** added keepalives (release dates are absent from the history).

Sources: [Prompt caching](https://aider.chat/docs/usage/caching.html), [configuration options](https://aider.chat/docs/config/options.html#cache-settings), [HISTORY: v0.51.0](https://aider.chat/HISTORY.html#aider-v0510), [HISTORY: v0.53.0](https://aider.chat/HISTORY.html#aider-v0530).

### 3. Edit verification is explicit and configurable, not synonymous with correctness

The reference documents `--lint-cmd`, auto-lint (default true), `--test-cmd`, and auto-test (default false); `/test` runs configured tests and can feed failures back for repair. **Certainty:** High for the documented command contract; Aider single-source. **Applicability:** expose which checks were selected, run, and passed, rather than infer verification from an edit or agent response. IssueOps records `verify_argv`, attempt evidence, and stop status but does not execute loop checks (`.issueops/architecture/runtime.md`, “실행 모드”). **Counterevidence:** this does not prove tests are sufficient, deterministic, or exhaustive; IssueOps intentionally has a different execution boundary.

Sources: [configuration: linting and testing](https://aider.chat/docs/config/options.html#fixing-and-committing), [commands](https://aider.chat/docs/usage/commands.html).

### 4. Release evidence supports visibility, not quantified performance

**v0.86.1 (2025-08-13)** notes GPT-5 reasoning settings, not maps or edit checks. **v0.85.0** claims faster history summarization and skipped expensive file tracking under `--skip-sanity-check-repo`, without measured effects. Docs expose map/cache controls, `/map`, `/tokens`, `/diff`, `/lint`, `/test`. **Certainty:** High these features/claims are published; performance is unquantified. **Applicability:** borrow measures (recompute count, map tokens, cache-hit visibility, check command/result), not speed estimates. **Counterevidence:** no benchmark or independent corroboration.

Sources: [HISTORY: v0.85.0 and v0.86.1](https://aider.chat/HISTORY.html#aider-v0850), [configuration options](https://aider.chat/docs/config/options.html), [commands](https://aider.chat/docs/usage/commands.html).

## EXPAND

- Measure IssueOps context assembly by elapsed time, bytes/tokens emitted, and repeated-read/recompute counts on representative repositories before proposing caching.
- Compare those results with existing docs indexing and bounded MCP outputs; keep reusable static project-doc catalog separate from runtime caches, per `.issueops/architecture/runtime.md` (“Docs / state / config / logs”).
- If edit verification visibility is the gap, expose the recorded command, exit status, and evidence as distinct fields; do not imply IssueOps executes loop checks.
- No independent Aider implementation or performance corroboration was established.

## Retrieval notes

- `https://aider.chat/docs/usage/linting.html` returned HTTP 404 and was not used; the official configuration reference and command documentation supplied the lint/test evidence.
- A `webfetch` request for `https://api.github.com/repos/Aider-AI/aider/releases/tags/v0.86.1` with `format="json"` was rejected because webfetch accepts markdown, text, or html. Retrying the same URL with `format="text"` returned HTTP 404 (no GitHub release object); PyPI metadata and the official version-bump commit verified the package date instead.
- All public URLs cited above were reopened on 2026-10-02. The cited IssueOps runtime sections were read from `.issueops/architecture/runtime.md`.
