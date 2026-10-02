package issueopsapp

import (
	"context"
	"io"

	"issueops/internal/adapter/outbound/issueopsrecord"
)

// issueOpsMCPTraceBinding binds only this request's traceparent (clearing any
// inherited correlation) and then arms the record span observer, so the
// lease/completion stores that open sqlstore spans directly on the MCP path
// emit correlated span events like the CLI runtime does.
func issueOpsMCPTraceBinding(writer io.Writer) func(context.Context, string) (context.Context, bool) {
	store := issueOpsRecordStore("mcp", issueOpsRecordObserver(writer))
	return func(ctx context.Context, raw string) (context.Context, bool) {
		ctx, valid := issueopsrecord.WithTraceparent(ctx, raw)
		return store.ObserveSpans(ctx, "request"), valid
	}
}
