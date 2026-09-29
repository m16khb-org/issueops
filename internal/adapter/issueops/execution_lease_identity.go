package issueops

import (
	"path/filepath"
	"strings"

	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
)

// ValidateNativeActorProcess applies the same ancestry and live-process receipt
// checks used by lease transitions before preparation can persist a new holder.
func ValidateNativeActorProcess(actor issueops.NativeActor) error {
	_, err := cycleapp.NormalizeNativeActor(actor, inspectNativeProcessReceipt)
	return err
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
