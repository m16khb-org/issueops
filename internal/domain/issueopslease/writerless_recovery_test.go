package issueopslease

import "testing"

func TestWriterlessRecoveryActionMatrix(t *testing.T) {
	tests := []struct {
		status string
		want   WriterlessRecoveryAction
	}{
		{"claimable", RecoveryClaim},
		{"released", RecoveryReplacePreview},
		{"revoking", RecoveryFinalizePreview},
		{"active", RecoveryNone},
	}
	for _, test := range tests {
		if got := DecideWriterlessRecovery(test.status); got != test.want {
			t.Errorf("%s: got %q want %q", test.status, got, test.want)
		}
	}
}
