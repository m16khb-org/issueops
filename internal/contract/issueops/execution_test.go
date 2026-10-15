package issueops_test

import (
	"strings"
	"testing"

	. "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

func validExecutionForTest() Execution {
	return Execution{
		Workspace: Workspace{
			SourceRoot: "/repo",
			Root:       "/repo.worktrees/run",
			Branch:     "run",
			BaseHead:   strings.Repeat("a", 40),
			LinkedAt:   "2026-07-30T00:00:00Z",
		},
		Lease: WriteLease{Generation: 2, Status: LeaseStatusReleased},
	}
}

func TestValidateNativeActorAcceptsOmo(t *testing.T) {
	err := issueopsdomain.ValidateNativeActor(NativeActor{
		Host:      "omo",
		SessionID: "019ff5b8-7d62-707a-a693-5e7a5e8a3187",
		SessionProcess: &NativeProcessReceipt{
			PID: 42, StartedAt: "2026-08-12T00:00:00Z", Executable: "/Users/test/Library/pnpm/bin/omo",
		},
	})
	if err != nil {
		t.Fatalf("Omo native actor must be valid: %v", err)
	}
}

func TestValidateExecutionAcceptsCompletionHistory(t *testing.T) {
	execution := validExecutionForTest()
	execution.CompletionHistory = []ExecutionCompletionHistory{{
		Generation: 1,
		Completion: ExecutionCompletion{
			Generation: 1, FinalHead: strings.Repeat("b", 40), VerificationReportPath: ".issueops/verified-execution/report.json",
			Verification: []string{"go test ./... -count=1"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/1", CompletedAt: "2026-08-03T00:00:00Z",
		},
		Reason: "new verified HEAD", ReopenedAt: "2026-08-04T00:00:00Z",
	}}
	if err := issueopsdomain.ValidateExecution(execution); err != nil {
		t.Fatalf("valid completion history rejected: %v", err)
	}
}

func TestValidateExecutionRejectsInvalidCompletionHistory(t *testing.T) {
	valid := ExecutionCompletionHistory{
		Generation: 1,
		Completion: ExecutionCompletion{
			Generation: 1, FinalHead: strings.Repeat("b", 40), VerificationReportPath: ".issueops/verified-execution/report.json",
			Verification: []string{"go test ./... -count=1"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/1", CompletedAt: "2026-08-03T00:00:00Z",
		},
		Reason: "new verified HEAD", ReopenedAt: "2026-08-04T00:00:00Z",
	}
	for _, test := range []struct {
		name   string
		mutate func(*ExecutionCompletionHistory)
	}{
		{name: "generation", mutate: func(entry *ExecutionCompletionHistory) { entry.Generation = 0 }},
		{name: "completion", mutate: func(entry *ExecutionCompletionHistory) { entry.Completion.Verification = nil }},
		{name: "blank verification", mutate: func(entry *ExecutionCompletionHistory) { entry.Completion.Verification = []string{" "} }},
		{name: "generation conflict", mutate: func(entry *ExecutionCompletionHistory) { entry.Completion.Generation = 2 }},
		{name: "current generation", mutate: func(entry *ExecutionCompletionHistory) {
			entry.Generation = 2
			entry.Completion.Generation = 2
		}},
		{name: "future generation", mutate: func(entry *ExecutionCompletionHistory) {
			entry.Generation = 3
			entry.Completion.Generation = 3
		}},
		{name: "reason", mutate: func(entry *ExecutionCompletionHistory) { entry.Reason = " " }},
		{name: "reopened at", mutate: func(entry *ExecutionCompletionHistory) { entry.ReopenedAt = " " }},
	} {
		t.Run(test.name, func(t *testing.T) {
			entry := valid
			test.mutate(&entry)
			execution := validExecutionForTest()
			execution.CompletionHistory = []ExecutionCompletionHistory{entry}
			if err := issueopsdomain.ValidateExecution(execution); err == nil {
				t.Fatal("invalid completion history accepted")
			}
		})
	}
}

func TestValidateExecutionRequiresWorkspaceIsolatedFromSourceRoot(t *testing.T) {
	execution := validExecutionForTest()
	if err := issueopsdomain.ValidateExecution(execution); err != nil {
		t.Fatalf("valid execution rejected: %v", err)
	}
	execution.Workspace.Root = execution.Workspace.SourceRoot
	if err := issueopsdomain.ValidateExecution(execution); err == nil || !strings.Contains(err.Error(), "isolated from source_root") {
		t.Fatalf("workspace root equal to source_root accepted: %v", err)
	}
}
