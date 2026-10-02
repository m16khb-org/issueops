package issueopsrecord

import (
	"context"

	tracedomain "issueops/internal/domain/trace"
)

type recordTraceContextKey struct{}

// WithTraceparent binds this request's correlation and clears inherited values
// when the header is absent or invalid. False requests a warning without the raw value.
func WithTraceparent(ctx context.Context, raw string) (context.Context, bool) {
	trace, valid := tracedomain.ParseTraceparent(raw)
	return context.WithValue(ctx, recordTraceContextKey{}, trace), raw == "" || valid
}
