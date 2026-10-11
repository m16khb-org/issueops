package issueops

import (
	"strings"
	"testing"
)

func TestValidateIssueCreateIntentRejectsUnboundedDurableFields(t *testing.T) {
	valid := IssueOpsIssueCreateIntent{
		OperationID:      "0123456789abcdef0123456789abcdef",
		Marker:           "<!-- issueops:issue-create:0123456789abcdef0123456789abcdef -->",
		Provider:         "github",
		ProjectAuthority: "github.com/acme/repo",
		Title:            "Title",
		BodySHA256:       strings.Repeat("a", 64),
		Status:           IssueCreateIntentInvokedUnknown,
		Attempt:          1,
		Failure:          "timeout",
		StartedAt:        "2026-08-14T00:00:00Z",
		UpdatedAt:        "2026-08-14T00:01:00Z",
	}
	if err := ValidateIssueCreateIntent(valid); err != nil {
		t.Fatalf("valid intent: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*IssueOpsIssueCreateIntent)
	}{
		{"title", func(intent *IssueOpsIssueCreateIntent) {
			intent.Title = strings.Repeat("t", MaxIssueCreateTitleBytes+1)
		}},
		{"failure", func(intent *IssueOpsIssueCreateIntent) {
			intent.Failure = strings.Repeat("f", MaxIssueCreateFailureBytes+1)
		}},
		{"canonical URL", func(intent *IssueOpsIssueCreateIntent) {
			intent.Status = IssueCreateIntentVerificationFailed
			intent.CanonicalURL = "https://github.com/acme/repo/issues/" + strings.Repeat("1", MaxIssueCreateURLBytes)
		}},
		{"labels", func(intent *IssueOpsIssueCreateIntent) { intent.Labels = make([]string, MaxIssueCreateValues+1) }},
		{"label value", func(intent *IssueOpsIssueCreateIntent) {
			intent.Labels = []string{strings.Repeat("l", MaxIssueCreateValueBytes+1)}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent := valid
			test.mutate(&intent)
			if err := ValidateIssueCreateIntent(intent); err == nil {
				t.Fatal("expected durable bound error")
			}
		})
	}
}

func TestValidateIssueCreateIntentRejectsInvalidIdentityAndStatus(t *testing.T) {
	const id = "0123456789abcdef0123456789abcdef"
	valid := IssueOpsIssueCreateIntent{
		OperationID:      id,
		Marker:           "<!-- issueops:issue-create:" + id + " -->",
		Provider:         "gitlab",
		ProjectAuthority: "gitlab.com/acme/repo",
		Title:            "Title",
		BodySHA256:       strings.Repeat("a", 64),
		Status:           IssueCreateIntentPending,
		Attempt:          1,
		StartedAt:        "2026-08-14T00:00:00Z",
		UpdatedAt:        "2026-08-14T00:01:00Z",
	}
	if err := ValidateIssueCreateIntent(valid); err != nil {
		t.Fatalf("valid intent: %v", err)
	}
	tests := []struct {
		name    string
		mutate  func(*IssueOpsIssueCreateIntent)
		wantErr string
	}{
		{"blank required field", func(i *IssueOpsIssueCreateIntent) { i.Title = "  " }, "title is required"},
		{"first blank field wins in declared order", func(i *IssueOpsIssueCreateIntent) { i.Marker, i.Title, i.UpdatedAt = "", "", "" }, "marker is required"},
		{"missing updated_at", func(i *IssueOpsIssueCreateIntent) { i.UpdatedAt = "" }, "updated_at is required"},
		{"short body digest", func(i *IssueOpsIssueCreateIntent) { i.BodySHA256 = strings.Repeat("a", 63) }, "body_sha256 must be 64 hex characters"},
		{"short operation id", func(i *IssueOpsIssueCreateIntent) { i.OperationID = id[:31] }, "operation_id must be 32 lowercase hexadecimal characters"},
		{"uppercase operation id", func(i *IssueOpsIssueCreateIntent) { i.OperationID = strings.ToUpper(id) }, "operation_id must be 32 lowercase hexadecimal characters"},
		{"non-hex operation id", func(i *IssueOpsIssueCreateIntent) { i.OperationID = strings.Repeat("z", 32) }, "operation_id must be 32 lowercase hexadecimal characters"},
		{"non-hex body digest", func(i *IssueOpsIssueCreateIntent) { i.BodySHA256 = strings.Repeat("z", 64) }, "body_sha256 must be lowercase hexadecimal"},
		{"uppercase body digest", func(i *IssueOpsIssueCreateIntent) { i.BodySHA256 = strings.Repeat("A", 64) }, "body_sha256 must be lowercase hexadecimal"},
		{"provider", func(i *IssueOpsIssueCreateIntent) { i.Provider = "bitbucket" }, "provider is invalid"},
		{"project authority size", func(i *IssueOpsIssueCreateIntent) {
			i.ProjectAuthority = strings.Repeat("p", MaxIssueCreateAuthorityBytes+1)
		}, "project_authority exceeds"},
		{"timestamp size", func(i *IssueOpsIssueCreateIntent) { i.StartedAt = strings.Repeat("t", 65) }, "timestamps exceed 64 bytes"},
		{"assignee count", func(i *IssueOpsIssueCreateIntent) { i.Assignees = make([]string, MaxIssueCreateValues+1) }, "assignees exceeds"},
		{"marker mismatch", func(i *IssueOpsIssueCreateIntent) { i.Marker = "<!-- issueops:issue-create:other -->" }, "marker does not match operation_id"},
		{"attempt zero", func(i *IssueOpsIssueCreateIntent) { i.Attempt = 0 }, "attempt must be positive"},
		{"unknown status", func(i *IssueOpsIssueCreateIntent) { i.Status = "exploded" }, `unsupported issue create intent status "exploded"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent := valid
			test.mutate(&intent)
			err := ValidateIssueCreateIntent(intent)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestValidateIssueCreateIntentAcceptsEveryStatus(t *testing.T) {
	const id = "0123456789abcdef0123456789abcdef"
	for _, status := range []string{
		IssueCreateIntentPending, IssueCreateIntentNotInvoked, IssueCreateIntentInvokedUnknown,
		IssueCreateIntentURLObserved, IssueCreateIntentVerificationFailed,
		IssueCreateIntentReceiptFailed, IssueCreateIntentCompleted,
	} {
		intent := IssueOpsIssueCreateIntent{
			OperationID: id, Marker: "<!-- issueops:issue-create:" + id + " -->",
			Provider: "github", ProjectAuthority: "a", Title: "t",
			BodySHA256: strings.Repeat("0", 64), Status: status, Attempt: 2,
			StartedAt: "s", UpdatedAt: "u",
		}
		if err := ValidateIssueCreateIntent(intent); err != nil {
			t.Errorf("status %q: %v", status, err)
		}
	}
}
