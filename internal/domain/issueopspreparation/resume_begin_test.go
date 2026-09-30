package issueopspreparation

import (
	"strings"
	"testing"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestBuildResumeIntentSealsOwnerIdentityAndStage(t *testing.T) {
	record := preparationcontract.Record{
		ID: "io-1", Repo: "/repo", IssueURL: "https://github.com/example/repo/issues/193",
		BranchPrepare: []byte(`{"provider":"github","issue_url":"https://github.com/example/repo/issues/193","link_verified":true}`),
		Execution: &preparationcontract.Execution{
			Mode: "orca", Workspace: preparationcontract.Workspace{SourceRoot: "/repo", Root: "/repo.worktrees/193-fix", Branch: "193-fix", BaseHead: "base"},
			Lease: preparationcontract.Lease{Generation: 4, Status: "claimable", ClaimTokenSHA256: strings.Repeat("b", 64)},
			Orca:  &preparationcontract.OrcaBinding{RuntimeID: "old-runtime", RepoID: "repo-id", WorktreeID: "tree-id", OwnerHost: "codex", OwnerModel: "model", OwnerEffort: "high", TaskID: "task", DispatchID: "dispatch", LeaseGeneration: 4},
		},
	}
	input := ResumeIntentInput{
		Record: record, Workspace: preparationcontract.WorkspaceRequest{LifecycleID: record.ID, SourceRoot: "/repo", Root: "/repo.worktrees/193-fix", Branch: "193-fix", BaseHead: "base", Confirm: true},
		Artifacts: preparationcontract.ResumeArtifacts{IssueBodySHA256: strings.Repeat("a", 64), OwnerPromptPath: "/prompt", OwnerPromptSHA256: strings.Repeat("c", 64), ContextPacketPath: "/packet", ContextPacketSHA256: strings.Repeat("d", 64)},
		RuntimeID: "new-runtime", OperationID: strings.Repeat("e", 32), StartedAt: "2026-07-31T03:15:00Z",
	}
	intent, err := BuildResumeIntent(input)
	if err != nil {
		t.Fatal(err)
	}
	if intent.Stage != preparationcontract.IntentStageTerminal || intent.Prepared.RuntimeID != "new-runtime" || intent.ResumeLease.Generation != 4 || intent.PriorBinding.RuntimeID != "old-runtime" || intent.Probe.Issue != 193 || intent.Marker == "" {
		t.Fatalf("intent=%+v", intent)
	}
	input.ReusedTerminalPTYID = "pty-old"
	intent, err = BuildResumeIntent(input)
	if err != nil {
		t.Fatal(err)
	}
	if intent.Stage != preparationcontract.IntentStageRun || intent.TerminalPTYID != "pty-old" {
		t.Fatalf("reused terminal intent=%+v", intent)
	}
	next := ApplyResumeIntent(record, intent)
	if next.Execution.Pending == nil || next.Execution.Pending.Kind != "owner_launch" || next.Execution.Pending.Marker != intent.Marker || record.Execution.Pending != nil {
		t.Fatalf("next=%+v original=%+v", next.Execution, record.Execution)
	}
}
