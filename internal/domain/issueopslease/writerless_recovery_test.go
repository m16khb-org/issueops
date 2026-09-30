package issueopslease

import "testing"

func TestWriterlessRecoveryActionMatrix(t *testing.T) {
	tests := []struct {
		status, mode string
		identity     bool
		want         WriterlessRecoveryAction
	}{
		{"claimable", "orca", true, RecoveryResume},
		{"claimable", "orca", false, RecoveryReplacePreview},
		{"claimable", "direct", false, RecoveryDirectClaim},
		{"released", "orca", false, RecoveryReplacePreview},
		{"revoking", "direct", false, RecoveryFinalizePreview},
		{"active", "direct", false, RecoveryNone},
	}
	for _, test := range tests {
		if got := DecideWriterlessRecovery(test.status, test.mode, test.identity); got != test.want {
			t.Errorf("%s/%s identity=%t: got %q want %q", test.status, test.mode, test.identity, got, test.want)
		}
	}
}
