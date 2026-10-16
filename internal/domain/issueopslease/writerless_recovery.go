package issueopslease

type WriterlessRecoveryAction string

const (
	RecoveryNone            WriterlessRecoveryAction = ""
	RecoveryReplacePreview  WriterlessRecoveryAction = "replace_preview"
	RecoveryClaim           WriterlessRecoveryAction = "claim"
	RecoveryFinalizePreview WriterlessRecoveryAction = "finalize_preview"
)

func DecideWriterlessRecovery(status string) WriterlessRecoveryAction {
	switch status {
	case "claimable":
		return RecoveryClaim
	case "released":
		return RecoveryReplacePreview
	case "revoking":
		return RecoveryFinalizePreview
	}
	return RecoveryNone
}
