package issueopslease

import "testing"

func TestWriterlessRecoveryActionMatrix(t *testing.T) {
	tests := []struct {
		status, mode string
		want         WriterlessRecoveryAction
	}{
		{"claimable", "orca", RecoveryResume},
		{"claimable", "direct", RecoveryDirectClaim},
		{"released", "orca", RecoveryReplacePreview},
		{"revoking", "direct", RecoveryFinalizePreview},
		{"active", "direct", RecoveryNone},
	}
	for _, test := range tests {
		if got := DecideWriterlessRecovery(test.status, test.mode); got != test.want {
			t.Errorf("%s/%s: got %q want %q", test.status, test.mode, got, test.want)
		}
	}
}
