package commandparse

import (
	"testing"

	commandparsecontract "issueops/internal/contract/commandparse"
)

func TestValidateGeneratedCommandInvocation(t *testing.T) {
	expected := commandparsecontract.GeneratedCommandProvenance{
		ExecutablePath:   "/worktree/bin/issueops",
		ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		LeaseGeneration:  7,
	}
	for _, test := range []struct {
		name     string
		observed commandparsecontract.GeneratedCommandProvenance
		durable  uint64
		wantCode string
	}{
		{name: "matched", observed: expected, durable: 7},
		{name: "stale generation", observed: expected, durable: 8, wantCode: "generated_command_generation_mismatch"},
		{name: "stale binary", observed: commandparsecontract.GeneratedCommandProvenance{
			ExecutablePath: "/installed/bin/issueops", ExecutableSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", LeaseGeneration: 7,
		}, durable: 7, wantCode: "generated_command_binary_provenance_mismatch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateGeneratedCommandInvocation(expected, test.observed, test.durable)
			if test.wantCode == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			fields, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
			if !ok || fields.IssueOpsErrorFields()["code"] != test.wantCode {
				t.Fatalf("error = %T %v, want code %s", err, err, test.wantCode)
			}
		})
	}
}
