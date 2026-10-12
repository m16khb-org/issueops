package commandparse

import "testing"

func TestParseExactIssueOpsCommandAcceptsBareLifecycleCommand(t *testing.T) {
	command, ok := ParseExactIssueOpsCommand("issueops status")
	if !ok || command.Path != "status" || command.Start != 2 {
		t.Fatalf("bare lifecycle command = %+v, ok=%v", command, ok)
	}
}

func TestParseExactIssueOpsArgsAcceptsSingleArgument(t *testing.T) {
	command, ok := ParseExactIssueOpsArgs([]string{"status"})
	if !ok || command.Path != "status" || command.Start != 2 || len(command.Tokens) != 2 {
		t.Fatalf("single-argument argv = %+v, ok=%v", command, ok)
	}
}

func TestExactFlagsRejectsValueFlagWithoutValueAtEnd(t *testing.T) {
	values := map[string]bool{"--id": true}
	command := ExactIssueOpsCommand{Tokens: []string{"issueops", "status", "--id"}, Start: 2}
	if flags, ok := ExactFlags(command, values, nil, nil); ok {
		t.Fatalf("trailing value flag without value accepted: %v", flags)
	}
	command.Tokens = []string{"issueops", "status", "--id", "io-1"}
	flags, ok := ExactFlags(command, values, nil, nil)
	if !ok || len(flags["--id"]) != 1 || flags["--id"][0] != "io-1" {
		t.Fatalf("value flag consuming the last token = %v, ok=%v", flags, ok)
	}
}

func TestParseExactIssueOpsArgsGroupCommandWithoutSubcommandDoesNotPanic(t *testing.T) {
	command, ok := ParseExactIssueOpsArgs([]string{"execution"})
	if ok && command.Start != len(command.Tokens) {
		t.Fatalf("accepted group command = %+v, want Start == len(Tokens)", command)
	}
}
