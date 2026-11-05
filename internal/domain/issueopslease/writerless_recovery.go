package issueopslease

// WriterlessRecoveryAction names the next step to recover a lease that has no writer.
type WriterlessRecoveryAction string

// RecoveryReplacePreview, RecoveryClaim and RecoveryFinalizePreview are the recovery actions
// DecideWriterlessRecovery can return.
const (
	recoveryNone            WriterlessRecoveryAction = ""
	RecoveryReplacePreview  WriterlessRecoveryAction = "replace_preview"
	RecoveryClaim           WriterlessRecoveryAction = "claim"
	RecoveryFinalizePreview WriterlessRecoveryAction = "finalize_preview"
)

// DecideWriterlessRecovery maps a lease status to the recovery action: claimable claims, released previews
// a replace, revoking previews a finalize. Any other status needs no action and yields "".
func DecideWriterlessRecovery(status string) WriterlessRecoveryAction {
	switch status {
	case "claimable":
		return RecoveryClaim
	case "released":
		return RecoveryReplacePreview
	case "revoking":
		return RecoveryFinalizePreview
	}
	return recoveryNone
}
