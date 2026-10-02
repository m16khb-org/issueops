# Issueops MCP descriptor payload and discovery

Retrieval date: **2026-10-02**. Model: `gpt-6.1-sol` (`PI_MODEL`). Scope: repository implementation, neighboring tests, official public sources, and the lead-owned runtime baseline; no new probe, benchmark, installation, or production change.

## Versions and provenance

`go.mod:31-39` pins MCP Go SDK **v1.6.1**. Official release metadata records publication on **2026-05-22T11:52:56Z**: https://api.github.com/repos/modelcontextprotocol/go-sdk/releases/tags/v1.6.1. Its release notes, https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.6.1, concern an HTTP Content-Type escape hatch, not descriptor optimization. These are same-producer sources, not independent corroboration.

The supplied `.issueops/research/agent-updates-2026-10-02/mcp-baseline.md:5-28` observes issueops **0.1.0**, protocol revision **2025-11-25**, and repository SHA `02be6d78cb0209c59cd0099614d9a03e453a59b6`. The versioned official tool specification is https://modelcontextprotocol.io/specification/2025-11-25/server/tools. The revision date is not an independently established issueops release date; none is inferred.

## Findings

### 1. Full discovery aggregates capability catalogs, not just IssueOpsBasicTools

`internal/adapter/inbound/catalog/mcp/catalog.go:18-65` assembles twelve advertised sections and one non-advertised alias section. Advertisement and dispatch derive from the same ordered section list, but dispatch includes aliases omitted from discovery. `cmd/issueops/contractgolden/contract_golden_test.go:22-24` snapshots the assembled descriptor maps.

The lead-owned baseline reports **51 tools and 30,888 UTF-8 bytes** of compact tool-array JSON, excluding the JSON-RPC envelope. This is one sample, not tokens or proof that every turn includes the complete catalog. Its uncontrolled initialize/list timings do not establish a bottleneck.

**Certainty:** high for source structure; measured runtime facts are attributed to the lead, not rerun. **Applicability:** evaluate the full catalog when considering deferred discovery or payload reduction. **Counterevidence:** stable source assembly order does not prove SDK wire ordering, and non-advertised aliases must not be accidentally exposed by a discovery redesign.

### 2. Descriptor metadata is deliberately narrow; structured outputs are not supplied

`internal/contract/mcp/catalog_types.go:3-8` contains only name, description, and inputSchema. `catalog.go:70-80` projects those fields; `cmd/issueops/mcpcli/mcp_sdk_server.go:134-152` registers them through SDK AddTool. The neighboring `catalog_assembly_test.go:26-51` checks preservation of that shape.

The official 2025-11-25 specification permits optional title, annotations, outputSchema, icons, and execution metadata. Issueops does not populate these through this path. Its ordinary response branch serializes JSON into TextContent (`mcp_sdk_server.go:124-130`), rather than supplying structuredContent.

**Certainty:** high for inspected registration and response paths; the protocol claim is official single-source evidence. **Applicability:** output schemas and accurate behavioral metadata are visibility/interoperability candidates. **Counterevidence:** optional-field absence is not a protocol violation; adding fields increases payload, and outputSchema would require matching structured results, not merely documentation.

### 3. Discovery is server-instance scoped, with validation before effects

`mcp_sdk_server.go:22-32,134-152` constructs a server and registers its supplied catalog at initialization. Argument validation precedes handler invocation (`:81-94`). Neighboring tests use incompatible schemas to detect shared catalog leakage and effects-before-validation (`cmd/issueops/mcpcli/catalog_isolation_test.go:15-56`); `:59-88` checks each server's advertised schema and resources. The in-process transport test also checks non-empty listing without daemon files (`mcp_transport_test.go:21-42`).

**Certainty:** high for implementation and test assertions; tests were read, not executed. **Applicability:** preserve instance ownership when considering cached schema construction. **Counterevidence:** these tests do not measure allocation costs or prove a cache would help. SDK registration, rather than a custom per-request search handler here, owns list discovery.

### 4. Binary-schema cache invalidation already has an installer mitigation

`.issueops/CAUTIONS.md:41-44` documents the mitigation. `cmd/issueops/issueopsapp/host_installers.go:67-77` wires SHA-256 of the full advertised catalog into Omo installation. `internal/adapter/omo/mcp.go:47-65` rejects absent/empty digests and writes `ISSUEOPS_MCP_CATALOG_SHA256` into server env. `install_test.go:198-222` checks this against the assembled catalog; `:39-47` applies the assertion to user and project configurations.

**Certainty:** high for existing code, not current user configuration. **Applicability:** do not propose this as a missing feature. **Counterevidence:** `.issueops/operations/install.md:52-60` requires installation/update to refresh the token and reconnect to apply new binary behavior. A cached descriptor is not connected readiness; schema hashing is not executable-content hashing.

## EXPAND

- **EXPAND-PAYLOAD:** measure actual host-selected schemas and prompt usage before proposing descriptor trimming or deferred exposure.
- **EXPAND-STRUCTURED:** assess representative output contracts and client support before adding outputSchema/structuredContent.
- **EXPAND-INVALIDATION:** verify update-driven token change and lazy reconnect in an isolated fixture; distinguish cached catalog availability from live readiness.

Verification: key URLs reopened and cited code reread. `git diff --check -- .issueops/research/agent-updates-2026-10-02/issueops-01.md` exited 0; because the report is untracked, its content also received manual review. No inaccessible public source. Failed lookup: reading nonexistent `internal/contract/mcp/tool.go` returned ENOENT; recovered using `catalog_types.go`. A search included nonexistent `internal/adapter/host`; recovered with concrete Omo paths. No failed build/test command: diagnostics, tests, and builds were excluded for this evidence-only lane.
