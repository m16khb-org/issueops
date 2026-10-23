---
name: 2026-10-24-mcp-http-readtimeout-and-test-tmpdir-leaks
description: Caution record for a solved false case or recurring risk.
---

# net/http ReadTimeout cancels long MCP tool calls; tests must not leak TMPDIR

- Date: 2026-10-24
- Kind: `caution`
- Source: autoresearch 2026-10-23 usability session
- Summary: A connection read deadline that expires while a handler runs cancels the request context, so the shared MCP HTTP server sets only ReadHeaderTimeout and IdleTimeout. Test TempDir hygiene: a TestMain MkdirTemp must be removed after m.Run, and a subtest with an empty name leaves an orphan t.TempDir parent.
- Context: Resource audit 2026-10-23: TMPDIR held 2,866 entries (1,075 issueops-xdg-* from cmd/issueops/issueopsapp/wire_test.go TestMain, 420 issueops-state-roundtrip-*). TMPDIR=<scratch> go test ./... left 3 entries per run. Adding ReadTimeout to cmd/issueops/mcpcli/mcp_http.go was considered and rejected: net/http backgroundRead turns the expired deadline into cancelCtx for the running handler, and gates_check legitimately runs for minutes.
- Resolution: mcp_http.go sets IdleTimeout 2m only (no Read/WriteTimeout). wire_test.go removes the XDG dir after m.Run; TestPublicStateUnknownSchemaIsRefusedWithoutRewriting names its subtests. Re-check leaks with: T=$(mktemp -d); TMPDIR=$T go test -count=1 ./...; ls $T (expect empty).
