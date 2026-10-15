package issueopspreparation

import (
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestDecisionMatrix(t *testing.T) {
	tests := []struct {
		name      string
		confirm   bool
		execution *leasecontract.Execution
		want      Code
	}{
		{name: "new preview", want: CodePreview},
		{name: "new confirm", confirm: true, want: CodeApply},
		{name: "pending precedes writerless", confirm: true, execution: preparedExecution("claimable", false, true), want: CodePendingReconcile},
		{name: "pending on preview", execution: preparedExecution("active", true, true), want: CodePendingReconcile},
		{name: "existing active", confirm: true, execution: preparedExecution("active", true, false), want: CodeExisting},
		{name: "claimable writerless", confirm: true, execution: preparedExecution("claimable", false, false), want: CodeWriterless},
		{name: "released writerless", confirm: true, execution: preparedExecution("released", false, false), want: CodeWriterless},
		{name: "revoking writerless", confirm: true, execution: preparedExecution("revoking", true, false), want: CodeWriterless},
		{name: "preview exposes writerless as existing", execution: preparedExecution("claimable", false, false), want: CodeExisting},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := Decide(DecisionInput{
				Command: preparationcontract.Command{ID: "io-prepare", Confirm: test.confirm},
				Snapshot: preparationcontract.Snapshot{
					Record:        leasecontract.Record{ID: "io-prepare", Execution: test.execution},
					CanonicalRoot: "/repo.worktrees/199-prepare",
				},
			})
			if decision.Code != test.want {
				t.Fatalf("decision=%+v want=%q", decision, test.want)
			}
		})
	}
}

func preparedExecution(status string, holder, pending bool) *leasecontract.Execution {
	execution := &leasecontract.Execution{Lease: leasecontract.Lease{Generation: 1, Status: status}}
	if holder {
		execution.Lease.Holder = &leasecontract.Actor{Host: "codex", SessionID: "session"}
	}
	if pending {
		execution.Pending = &leasecontract.ExternalIntent{OperationID: "operation", Kind: "remote_pr_create", Marker: "marker"}
	}
	return execution
}
