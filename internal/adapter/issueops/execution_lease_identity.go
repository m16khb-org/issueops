package issueops

import (
	"context"
	"path/filepath"
	"strings"

	authorityapp "issueops/internal/application/authority"
	"issueops/internal/contract/issueops"
	authorityport "issueops/internal/port/authority"
)

// NativeProcessInspector is the PID reuse-safe live process observation used by
// actor verification.
type NativeProcessInspector struct{}

func (NativeProcessInspector) Inspect(ctx context.Context, receipt issueops.NativeProcessReceipt) (string, issueops.NativeProcessReceipt, error) {
	if err := ctx.Err(); err != nil {
		return NativeProcessStatusUnknown, issueops.NativeProcessReceipt{}, err
	}
	return inspectNativeProcessReceipt(receipt)
}

// NativeActorVerifier proves callers only by observed native ancestry and the
// live session receipt. It has no grant store, so a capability-bound request
// fails closed instead of falling back to native identity.
func NativeActorVerifier() authorityport.ActorVerifier {
	return authorityapp.New(nil, NativeProcessInspector{}, nil, nil, nil, nil)
}

func sameNativeActor(a, b *issueops.NativeActor) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return sameNativeActorIdentity(a, b) && sameNativeProcessReceipt(a.SessionProcess, b.SessionProcess)
}

func sameNativeActorIdentity(a, b *issueops.NativeActor) bool {
	return a != nil && b != nil && strings.EqualFold(a.Host, b.Host) && a.SessionID == b.SessionID && a.AgentID == b.AgentID
}

func sameNativeProcessReceipt(a, b *issueops.NativeProcessReceipt) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func samePath(a, b string) bool {
	left, err := filepath.Abs(strings.TrimSpace(a))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(left); resolveErr == nil {
		left = resolved
	}
	right, err := filepath.Abs(strings.TrimSpace(b))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(right); resolveErr == nil {
		right = resolved
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func inspectNativeProcessReceiptForRollover(
	receipt issueops.NativeProcessReceipt,
	processSnapshot map[int]nativeProcessSnapshotEntry,
) (string, issueops.NativeProcessReceipt, error) {
	if processSnapshot != nil {
		return inspectNativeProcessReceiptFromSnapshot(receipt, processSnapshot)
	}
	return inspectNativeProcessReceipt(receipt)
}
