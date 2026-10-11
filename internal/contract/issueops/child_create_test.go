package issueops

import (
	"strings"
	"testing"
)

func validChildCreateOperation(operationID string) ChildCreateOperation {
	return ChildCreateOperation{
		IssueOpsIssueCreateIntent: IssueOpsIssueCreateIntent{
			OperationID:      operationID,
			Marker:           "<!-- issueops:child-create:" + operationID + " -->",
			Provider:         "github",
			ProjectAuthority: "github.com/acme/repo",
			Title:            "Child",
			BodySHA256:       strings.Repeat("a", 64),
			Status:           IssueCreateIntentPending,
			Attempt:          1,
			StartedAt:        "2026-08-14T00:00:00Z",
			UpdatedAt:        "2026-08-14T00:01:00Z",
		},
		Origin:        "explicit",
		RequestSHA256: strings.Repeat("b", 64),
		ParentURL:     "https://github.com/acme/repo/issues/1",
	}
}

func TestValidateChildCreateOperationsAcceptsEmptyAndLeasedAndUnleased(t *testing.T) {
	if err := ValidateChildCreateOperations(nil); err != nil {
		t.Fatalf("nil operations: %v", err)
	}
	leased := validChildCreateOperation("22222222222222222222222222222222")
	leased.Generation = 3
	leased.Holder = &NativeActor{}
	leased.CWD = "/work/tree"
	err := ValidateChildCreateOperations([]ChildCreateOperation{
		validChildCreateOperation("11111111111111111111111111111111"),
		leased,
	})
	if err != nil {
		t.Fatalf("valid operations: %v", err)
	}
}

func TestValidateChildCreateOperationsRejectsInvalidOperation(t *testing.T) {
	const id = "11111111111111111111111111111111"
	tests := []struct {
		name    string
		mutate  func(*ChildCreateOperation)
		wantErr string
	}{
		{"issue create shape", func(op *ChildCreateOperation) { op.Provider = "bitbucket" }, "provider is invalid"},
		{"issue-create marker instead of child-create marker", func(op *ChildCreateOperation) {
			op.Marker = "<!-- issueops:issue-create:" + id + " -->"
		}, "invalid child create operation identity"},
		{"unknown origin", func(op *ChildCreateOperation) { op.Origin = "automatic" }, "invalid child create operation identity"},
		{"blank parent url", func(op *ChildCreateOperation) { op.ParentURL = "  " }, "invalid child create operation identity"},
		{"oversized parent url", func(op *ChildCreateOperation) {
			op.ParentURL = strings.Repeat("u", MaxIssueCreateURLBytes+1)
		}, "invalid child create operation identity"},
		{"short request digest", func(op *ChildCreateOperation) { op.RequestSHA256 = strings.Repeat("b", 63) }, "invalid child request digest"},
		{"uppercase request digest", func(op *ChildCreateOperation) { op.RequestSHA256 = strings.Repeat("B", 64) }, "invalid child request digest"},
		{"non-hex request digest", func(op *ChildCreateOperation) { op.RequestSHA256 = strings.Repeat("g", 64) }, "invalid child request digest"},
		{"completed without canonical url", func(op *ChildCreateOperation) { op.Status = IssueCreateIntentCompleted }, "requires canonical_url"},
		{"url observed without canonical url", func(op *ChildCreateOperation) { op.Status = IssueCreateIntentURLObserved }, "requires canonical_url"},
		{"unleased with holder", func(op *ChildCreateOperation) { op.Holder = &NativeActor{} }, "invalid unleased child authority"},
		{"unleased with cwd", func(op *ChildCreateOperation) { op.CWD = "/work/tree" }, "invalid unleased child authority"},
		{"leased without holder", func(op *ChildCreateOperation) {
			op.Generation = 1
			op.CWD = "/work/tree"
		}, "missing child authority"},
		{"leased without cwd", func(op *ChildCreateOperation) {
			op.Generation = 1
			op.Holder = &NativeActor{}
		}, "missing child authority"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			op := validChildCreateOperation(id)
			test.mutate(&op)
			err := ValidateChildCreateOperations([]ChildCreateOperation{op})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestValidateChildCreateOperationsAcceptsCanonicalURLWhenCompleted(t *testing.T) {
	op := validChildCreateOperation("11111111111111111111111111111111")
	op.Status = IssueCreateIntentCompleted
	op.CanonicalURL = "https://github.com/acme/repo/issues/2"
	if err := ValidateChildCreateOperations([]ChildCreateOperation{op}); err != nil {
		t.Fatalf("completed with canonical url: %v", err)
	}
}

func TestValidateChildCreateOperationsRejectsDuplicateIdentity(t *testing.T) {
	const duplicate = "duplicate child create operation identity"
	first := validChildCreateOperation("11111111111111111111111111111111")
	second := validChildCreateOperation("22222222222222222222222222222222")

	t.Run("same operation id", func(t *testing.T) {
		dup := first
		dup.RequestSHA256 = strings.Repeat("c", 64)
		err := ValidateChildCreateOperations([]ChildCreateOperation{first, dup})
		if err == nil || err.Error() != duplicate {
			t.Fatalf("error = %v, want %q", err, duplicate)
		}
	})
	t.Run("implicit operations sharing a request digest", func(t *testing.T) {
		a, b := first, second
		a.Origin, b.Origin = "implicit", "implicit"
		err := ValidateChildCreateOperations([]ChildCreateOperation{a, b})
		if err == nil || err.Error() != duplicate {
			t.Fatalf("error = %v, want %q", err, duplicate)
		}
	})
	t.Run("explicit operations may share a request digest", func(t *testing.T) {
		if err := ValidateChildCreateOperations([]ChildCreateOperation{first, second}); err != nil {
			t.Fatalf("explicit operations sharing a digest: %v", err)
		}
	})
	t.Run("implicit and explicit may share a request digest", func(t *testing.T) {
		a := first
		a.Origin = "implicit"
		if err := ValidateChildCreateOperations([]ChildCreateOperation{a, second}); err != nil {
			t.Fatalf("implicit + explicit sharing a digest: %v", err)
		}
	})
}
