// Package authoritytest holds test doubles for the authority port.
package authoritytest

import (
	"context"

	model "issueops/internal/contract/issueops"
)

// ProcessInspectorFunc adapts a function to authority.ProcessInspector.
type ProcessInspectorFunc func(context.Context, model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error)

func (f ProcessInspectorFunc) Inspect(ctx context.Context, receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
	return f(ctx, receipt)
}
