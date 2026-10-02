# MCP Streamable HTTP: current stable contract and compatibility

**Checked:** 2026-10-02. **Conclusion:** MCP `2026-07-28` is the latest stable specification. It replaces the `2025-11-25` Streamable HTTP contract with a breaking, stateless HTTP binding. Do not describe 2025 behavior as current. The live `/draft` transport page fetched on this date matched the stable page on the checked breaking points, but `/draft` is mutable and is not the released contract.

## Evidence and sources

- MCP [official llms.txt](https://modelcontextprotocol.io/llms.txt), fetched 2026-10-02: indexes stable `2026-07-28`, historical `2025-11-25`, and `/draft` as separate specification paths.
- MCP [2026-07-28 release announcement](https://blog.modelcontextprotocol.io/posts/2026-07-28/), published 2026-07-28; [stable Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http), [versioning](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning), and [subscriptions](https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/subscriptions), fetched 2026-10-02.
- Historical comparison: [2025-11-25 transports](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports), fetched 2026-10-02; deprecated [2024-11-05 HTTP with SSE](https://modelcontextprotocol.io/specification/2024-11-05/basic/transports), fetched 2026-10-02.
- Current mutable [draft Streamable HTTP](https://modelcontextprotocol.io/specification/draft/basic/transports/streamable-http), fetched 2026-10-02. Compared directly with stable for POST/GET, cancellation, sessions, headers, and replay; no difference was observed in those passages. This is only a dated snapshot.
- Client documentation, fetched 2026-10-02: [Claude Code MCP](https://docs.anthropic.com/en/docs/claude-code/mcp), [Codex MCP](https://developers.openai.com/codex/mcp/), and [Omo Native docs](https://omo.dev/docs).
- Local implementation evidence: `cmd/issueops/mcpcli/mcp_sdk_server.go:257-271`; `internal/adapter/doctor/checks.go:140-157`; `go.mod:33`; recorded live stdio observation in `.issueops/research/agent-updates-2026-10-02/mcp-baseline.md:7-20`.

## Verified protocol differences

**POST, GET, and SSE.** In `2026-07-28`, one MCP endpoint accepts POST. Each client request/notification is a POST; a request response can be JSON **or** request-scoped SSE, and clients must handle both. Thus SSE is optional for ordinary request responses, not mandatory. Change subscriptions are a separate case: `subscriptions/listen` is a POST whose response is a long-lived SSE stream. The current binding removes the standalone GET stream. A modern-only server may answer GET/DELETE with 405; this is not the `2025-11-25` rule, where GET could open an independent server-to-client stream. Sources: stable transport “Sending Messages,” “Receiving Messages,” “Message Flow,” “Backward Compatibility”; subscriptions spec.

**Sessions and per-request metadata.** The new core removes `initialize`/`initialized` and `Mcp-Session-Id`; each request carries protocol version, client identity, and capabilities in `_meta`. `MCP-Protocol-Version` must match the body metadata. Required `Mcp-Method` mirrors `method`; `Mcp-Name` mirrors `params.name`/`params.uri` for specified methods. Servers processing bodies must reject mismatches with 400/`HeaderMismatch`. The announcement additionally describes `Mcp-Name` and header-based routing. Per-request metadata is not a session token. Application state can still be carried explicitly in tool arguments. Sources: stable transport “Request Metadata” and versioning; 2026-07-28 announcement.

**Replay, retry, and disconnect.** Stable `2026-07-28` explicitly says resumable SSE via `Last-Event-ID` is unsupported. It does not impose the old SSE `retry:`/reconnect-and-replay contract; application/HTTP retry policy must not be confused with stream resumption. Closing a request’s SSE response stream **MUST** be treated as cancellation; the server should stop work and must send no further messages for that request. This reverses `2025-11-25`, which says disconnection is not cancellation, recommends an explicit `notifications/cancelled`, and permits GET + `Last-Event-ID` replay; that revision also specifies obeying an SSE `retry` field before reconnecting. The old rules cannot be carried across revisions.

**Origin and authentication.** Current and old HTTP transport guidance requires servers to validate Origin on every incoming connection; when a supplied Origin is invalid, reject with 403. The header itself is not stated as mandatory. Local servers should bind loopback. The current spec says proper authentication **SHOULD** be implemented; it does not make a particular auth scheme a transport-level MUST. Do not conflate these SHOULD security recommendations with required request metadata headers.

**Legacy HTTP+SSE is a distinct older transport.** `2024-11-05` defines two endpoints: an SSE endpoint sends an `endpoint` event, and clients POST subsequent messages to the announced URI. It is deprecated; current docs recommend migration to Streamable HTTP. `2025-11-25` Streamable HTTP had already replaced that two-endpoint shape with one endpoint, but retained optional GET streams, optional sessions, and resumability. The two revisions must not be collapsed into “legacy SSE.”

## Client support and repository applicability

- **Claude Code:** Official docs support remote HTTP and identify `streamable-http` as an alias for `http`. They describe fallback to SSE-only endpoints in Claude Code v2.1.265 or later. This verifies HTTP transport selection and legacy fallback, **not** explicit conformance to the `2026-07-28` request metadata, stateless-era negotiation, or cancellation contract.
- **Codex:** Official docs call out Streamable HTTP servers and URL, headers, bearer-token, and OAuth configuration. They do not identify the protocol revision or attest to the new per-request headers/cancellation behavior.
- **Omo Native:** Its official docs describe configured remote MCP servers with a `url`, but the fetched documentation does not specify Streamable HTTP revision support. Treat revision compatibility as unverified, not as absent or present.

IssueOps currently exposes its MCP SDK server through `IOTransport` in `cmd/issueops/mcpcli/mcp_sdk_server.go:257-271`; the dependency is `go-sdk v1.6.1` (`go.mod:33`). That is not evidence of an HTTP listener. Separately, the recorded IssueOps binary stdio probe negotiated `2025-11-25` (`mcp-baseline.md:11-20`); it establishes that observed binary’s protocol version, not which HTTP transport a host selects or what other revisions it supports. The doctor’s distinct gateway liveness probe posts an `initialize` request hard-coded to `2025-03-26` and deliberately treats any HTTP status as evidence of a responding listener (`internal/adapter/doctor/checks.go:140-157`). Do not use that reachability check as a protocol conformance result. Keep **protocol revision**, **transport selection**, and **client/SDK compatibility** as separate dimensions.

## Counterevidence and interpretation

The release announcement says the updated Tier 1 TypeScript, Python, Go, and C# SDKs speak `2026-07-28`; this is evidence about those SDK releases, not every application embedding them or the three named host clients. Stable transport expressly preserves legacy interoperability through version-aware fallback, so “new spec is final” does not mean all peers have upgraded. Official host docs establish generic remote HTTP / Streamable HTTP configuration, but not revision-level interoperability. No source examined supports a performance superiority claim; none is made here.

## EXPAND

- Build a revision × transport × feature compatibility matrix by exact Claude Code, Codex, and Omo Native release, including a disposable test server that exercises headers, unsupported-version fallback, disconnect cancellation, and subscriptions.
- Determine whether IssueOps needs a separately designed HTTP inbound adapter at all; do not infer that from its stdio probe or doctor’s listener check. If needed, evaluate `go-sdk` support and state/actor invariants against both protocol eras before proposing a migration.
- Reconcile the wider agent-update research’s historical 2025 observations with this 2026-07-28 boundary. Keep measured tool visibility/latency work distinct; there is no transport benchmark here.

**Source fan-out:** MCP official index/spec/changelog-era pages and release announcement; first-party Claude Code, Codex, and Omo docs; local IssueOps source and recorded probe.
**Claim verification:** Protocol semantics confirmed against fetched normative stable pages and historical page; draft matched checked stable passages at retrieval; host transport support confirmed only at the documented level; host revision-level support unconfirmed.
**Access boundary:** All cited public pages returned readable content. No secrets, authentication state, or session transcripts were read. No source was blocked.
