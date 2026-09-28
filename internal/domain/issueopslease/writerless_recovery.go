package issueopslease

type WriterlessRecoveryAction string

const (
	RecoveryNone            WriterlessRecoveryAction = ""
	RecoveryReplacePreview  WriterlessRecoveryAction = "replace_preview"
	RecoveryResume          WriterlessRecoveryAction = "resume"
	RecoveryDirectClaim     WriterlessRecoveryAction = "direct_claim"
	RecoveryFinalizePreview WriterlessRecoveryAction = "finalize_preview"
)

func DecideWriterlessRecovery(status, mode string, orcaIdentityComplete bool) WriterlessRecoveryAction {
	switch status {
	case "claimable":
		switch mode {
		case "orca":
			if orcaIdentityComplete {
				return RecoveryResume
			}
			return RecoveryReplacePreview
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
