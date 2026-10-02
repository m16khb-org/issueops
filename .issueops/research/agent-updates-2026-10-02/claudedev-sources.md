# Claude.dev: Anthropic technical-writing site and agent-harness sources

Retrieved 2026-10-02. This note verifies Claude.dev as a technical-writing source; it is not the former “Claude Dev” VS Code extension.

## Finding and identity check

`claude.dev` is an Anthropic-operated technical blog. Its own [Terms](https://claude.dev/terms/) say, “claude.dev ... is operated by Anthropic, PBC,” and say the articles and other non-code site content belong to Anthropic or its licensors. The fetched article HTML independently labels its JSON-LD publisher as `Anthropic` (`https://www.anthropic.com`); the site exposes a canonical article URL, an RSS feed, and links to official Claude Code docs. This conclusion rests on the operator statement and article metadata, not the domain name.

The author attributions reinforce that status: the context-engineering, prompt-caching, skills, and workflow articles identify Thariq Shihipar as a member of technical staff at Anthropic / on the Claude Code team; the workflow post also names Sid Bidasaria. These are first-party technical accounts, not independent benchmarks.

The similarly named extension is a separate thing. The [Cline repository README](https://github.com/cline/cline) presents Cline as an IDE/terminal/desktop coding agent and links its VS Code extension using Marketplace ID `saoudrizwan.claude-dev`. The fetched [Marketplace listing](https://marketplace.visualstudio.com/items?itemName=saoudrizwan.claude-dev) identifies the product as Cline. That historical package identifier explains the name collision; it does not make `claude.dev` an extension page.

## Dated articles worth adding to the research set

The public [RSS feed](https://claude.dev/rss.xml), titled “claude.dev Blog,” describes its contents as articles, videos, and build logs from Anthropic developers. Dates below use each RSS `pubDate`, cross-checked against article `article:published_time` / BlogPosting metadata for the five central sources. The feed build date is 2026-10-01.

| Published | Article and relevance |
|---|---|
| 2026-10-01 | [Getting started with Claude Code mods](https://claude.dev/blog/getting-started-with-claude-code-mods/): hooks shipped in plugins; relevant to skill/plugin lifecycle and execution visibility. Feed summary only in this pass. |
| 2026-09-25 | [What a task costs on Opus 5.5](https://claude.dev/blog/what-a-task-costs-on-opus-5-5/), Addy Osmani: task cost depends on turns, cache reads, output/thinking, and model; explicitly calls figures illustrations and directs readers to measure real usage. Article HTML confirms author/date. |
| 2026-09-25 | [Using Claude Code: Spending your effort](https://claude.dev/blog/spending-your-effort/), Thariq Shihipar: effort settings and model-specific measured behavior; feed metadata only here. |
| 2026-07-24 | [The new rules of context engineering for Claude 5 generation models](https://claude.dev/blog/the-new-rules-of-context-engineering-for-claude-5-generation-models/), Thariq Shihipar: system context, CLAUDE.md, skills, memory, progressive disclosure, and simplifying over-prescriptive instructions. |
| 2026-06-03 | [Lessons from building Claude Code: How we use skills](https://claude.dev/blog/lessons-from-building-claude-code-how-we-use-skills/), Thariq Shihipar: skill categories, gotchas, folder references, progressive disclosure, trigger descriptions, and measuring usage. |
| 2026-06-02 | [A harness for every task: dynamic workflows in Claude Code](https://claude.dev/blog/a-harness-for-every-task-dynamic-workflows-in-claude-code/), Thariq Shihipar and Sid Bidasaria: task-specific orchestration, fan-out/synthesis, adversarial verification, and token budgets. It warns workflows can consume significantly more tokens and are best for complex, high-value work. |
| 2026-04-30 | [Lessons from building Claude Code: Prompt caching is everything](https://claude.dev/blog/lessons-from-building-claude-code-prompt-caching-is-everything/), Thariq Shihipar: stable prompt prefixes, deterministic tool ordering, state changes via messages, deferred tool schemas, and cache-safe compaction. |
| 2026-04-10 | [Seeing like an agent: how we design tools in Claude Code](https://claude.dev/blog/seeing-like-an-agent/), Thariq Shihipar: tool design and evaluation; feed metadata only here. |

The five central article pages were fetched as HTML or Markdown, not accepted from search snippets. Their metadata agrees with the feed dates and author attribution. The 2026-10-01, 2026-09-25 effort, and 2026-04-10 rows are title/date/summary-level feed leads rather than claims from a full article read.

## Applicability to issueops

- **Keep the MCP inventory measurable and stable.** The caching article recommends a static-first prefix and stable tool definitions/order; its tool-search example uses lightweight deferred stubs rather than changing the live set. In issueops, [`catalogSections`, `AdvertisedTools`, and `DispatchMap`](../../../internal/adapter/inbound/catalog/mcp/catalog.go#L18-L44) establish one ordered source for advertised tools and dispatch. A separate local observation recorded **51 tools / 30,888 bytes** for one stdio `tools/list` response ([measurement note](mcp-baseline.md)); bytes are not tokens, and this does not prove what any host includes in model context. If testing deferred loading, measure host prompt/context behavior and first-use latency while preserving stable ordering and dispatch contracts.
- **Keep transport questions separate from context loading.** The local [`mcp_stream_test.go`](../../../cmd/issueops/mcpcli/mcp_stream_test.go#L30-L36) exercises listing tools and reading a resource through stdio. “Streamable HTTP” concerns MCP transport; Claude.dev’s deferred tool-loading discussion concerns schema/context exposure. These sources provide no evidence that switching transport improves prompt cost or execution speed.
- **Treat dynamic workflows as a selective experiment, not a default.** The article’s own counterpoint is token/coordination overhead. For issueops, first define a bounded task and compare success/quality, wall time, tokens or proxy measurements, and coordination cost against the existing path. The existing research observation for MCP latency is a single, uncontrolled run, not a performance baseline ([measurement note](mcp-baseline.md)).
- **Prefer focused, progressively disclosed skill guidance.** The skill article recommends accumulating concrete gotchas, linking detailed references from a small entry point, and describing when a skill should trigger. This fits the repository’s shared `skills/` source of truth; it does not justify deleting project-specific rules or importing Claude Code-only workflow features into the Go core.
- **Use the cost article’s measurement discipline, not its example price claims.** It says to compare real tasks and usage. For issueops, record repeatable before/after outcomes under the same host, task, tool catalog, and environment; do not turn the post’s illustrative model prices, single benchmark, or claimed savings into issueops estimates.

## Counterevidence, access notes, and confidence

The plain homepage fetch rendered only a shell/search surface, so it was not used to establish identity or article coverage. Public `sitemap.xml`, RSS, server-rendered article HTML/Markdown, and Terms provided the needed evidence; no browser was required. Sitemap `lastmod` values were not treated as publication dates. No auth wall, CAPTCHA, or access-control block was encountered. The probed `https://cline.bot/blog/introducing-cline` returned “Post Not Found”; it is excluded as historical evidence. Current Cline repo and Marketplace sources corroborate the separate product identity.

The site’s claims about Anthropic-internal results (including prompt reductions, cache impact, and eval outcomes) remain first-party claims. The cost post itself labels examples as illustrative and recommends measuring each team’s own tasks. None establishes a quantitative benefit for issueops.

## EXPAND leads

- Fetch and read the 2026-10-01 mods article and 2026-09-25 effort article; compare current Claude Code docs for feature/version applicability.
- Read the cost article’s linked costs, prompt-caching, and effort docs, then design a repeatable issueops measurement protocol without logging prompt contents.
- Test MCP deferred loading separately from Streamable HTTP: compare per-host schema exposure, tool-discovery latency, cache/prefix stability, and behavior after list changes.
- Evaluate one bounded dynamic workflow against a static issueops task path, with a held-out task set and explicit quality/cost stop criteria.
