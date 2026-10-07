package issueopslease

type WriterlessRecoveryAction string

const (
	RecoveryNone            WriterlessRecoveryAction = ""
	RecoveryReplacePreview  WriterlessRecoveryAction = "replace_preview"
	RecoveryResume          WriterlessRecoveryAction = "resume"
	RecoveryDirectClaim     WriterlessRecoveryAction = "direct_claim"
	RecoveryFinalizePreview WriterlessRecoveryAction = "finalize_preview"
)

func DecideWriterlessRecovery(status, mode string) WriterlessRecoveryAction {
	switch status {
	case "claimable":
		switch mode {
		case "orca":
			return RecoveryResume
		case "direct":
			return RecoveryDirectClaim
		}
	case "released":
		return RecoveryReplacePreview
	case "revoking":
		return RecoveryFinalizePreview
	}
	return RecoveryNone
}
