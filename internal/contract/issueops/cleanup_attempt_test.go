package issueops

import (
	"strings"
	"testing"
)

func TestValidateCleanupAttempt(t *testing.T) {
	valid := CleanupAttempt{
		Operation: CleanupOperationFinish,
		Token:     strings.Repeat("a", 64),
		StartedAt: "2026-08-14T00:00:00.123456789Z",
	}
	if err := ValidateCleanupAttempt(nil); err != nil {
		t.Fatalf("nil attempt: %v", err)
	}
	for _, operation := range []CleanupOperation{CleanupOperationFinish, CleanupOperationAbandon, CleanupOperationRemoteBranch} {
		attempt := valid
		attempt.Operation = operation
		if err := ValidateCleanupAttempt(&attempt); err != nil {
			t.Fatalf("operation %q: %v", operation, err)
		}
	}
	tests := []struct {
		name    string
		mutate  func(*CleanupAttempt)
		wantErr string
	}{
		{"unknown operation", func(a *CleanupAttempt) { a.Operation = "purge" }, "operation is invalid"},
		{"empty operation", func(a *CleanupAttempt) { a.Operation = "" }, "operation is invalid"},
		{"short token", func(a *CleanupAttempt) { a.Token = strings.Repeat("a", 63) }, "token is invalid"},
		{"uppercase token", func(a *CleanupAttempt) { a.Token = strings.Repeat("A", 64) }, "token is invalid"},
		{"non-hex token", func(a *CleanupAttempt) { a.Token = strings.Repeat("z", 64) }, "token is invalid"},
		{"non-RFC3339 timestamp", func(a *CleanupAttempt) { a.StartedAt = "2026-08-14 00:00:00" }, "timestamp is invalid"},
		{"empty timestamp", func(a *CleanupAttempt) { a.StartedAt = "" }, "timestamp is invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attempt := valid
			test.mutate(&attempt)
			err := ValidateCleanupAttempt(&attempt)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}
