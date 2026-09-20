package executioncmd

import (
	"context"
	"reflect"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestExecutionHandoffCmuxCLIRequiresExplicitCompleteIdentity(t *testing.T) {
	var captured issueopscontract.ExecutionCmuxHandoffRequest
	printed := 0
	deps := Deps{
		StateRoot: func() string { return "/state" },
		HandoffCmux: func(_ context.Context, stateRoot string, request issueopscontract.ExecutionCmuxHandoffRequest) (issueopscontract.ExecutionCmuxHandoffResult, error) {
			if stateRoot != "/state" {
				t.Fatalf("state root=%q", stateRoot)
			}
			captured = request
			return issueopscontract.ExecutionCmuxHandoffResult{OK: true, ID: request.ID, Generation: request.Generation}, nil
		},
		PrintJSON: func(any) error { printed++; return nil },
	}
	args := []string{
		"handoff-cmux", "--id", "io-13", "--generation", "7",
		"--cmux-executable", "/Applications/cmux.app/Contents/Resources/bin/cmux", "--cmux-version", "0.64.10",
		"--socket", "/private/tmp/cmux.sock", "--window", "11111111-1111-4111-8111-111111111111",
		"--host", "omo", "--host-executable", "/opt/homebrew/bin/omo", "--model", "openai/gpt-5.6", "--effort", "xhigh",
		"--prompt-file", "/repo/worktree/.issueops/cmux-prompt", "--prompt-sha256", strings.Repeat("a", 64),
		"--material-sha256", strings.Repeat("b", 64), "--json",
	}
	if err := Run(args, deps); err != nil {
		t.Fatal(err)
	}
	want := issueopscontract.ExecutionCmuxHandoffRequest{
		ID: "io-13", Generation: 7, CmuxExecutable: "/Applications/cmux.app/Contents/Resources/bin/cmux", CmuxVersion: "0.64.10",
		SocketPath: "/private/tmp/cmux.sock", WindowID: "11111111-1111-4111-8111-111111111111",
		Host: "omo", HostExecutable: "/opt/homebrew/bin/omo", Model: "openai/gpt-5.6", Effort: "xhigh",
		PromptFile: "/repo/worktree/.issueops/cmux-prompt", PromptSHA256: strings.Repeat("a", 64), MaterialSHA256: strings.Repeat("b", 64),
	}
	if !reflect.DeepEqual(captured, want) || printed != 1 {
		t.Fatalf("request=%+v printed=%d", captured, printed)
	}
}

func TestExecutionHandoffCmuxCLIDoesNotInvokeOnIncompleteOrInventedIdentity(t *testing.T) {
	calls := 0
	deps := Deps{StateRoot: func() string { return "/state" }, HandoffCmux: func(context.Context, string, issueopscontract.ExecutionCmuxHandoffRequest) (issueopscontract.ExecutionCmuxHandoffResult, error) {
		calls++
		return issueopscontract.ExecutionCmuxHandoffResult{}, nil
	}}
	for _, args := range [][]string{
		{"handoff-cmux", "--id", "io-13"},
		{"handoff-cmux", "--id", "io-13", "--generation", "1", "--runtime-id", "invented"},
	} {
		if err := Run(args, deps); err == nil {
			t.Fatalf("incomplete cmux command accepted: %q", args)
		}
	}
	if calls != 0 {
		t.Fatalf("cmux handler called before complete validation: %d", calls)
	}
}
