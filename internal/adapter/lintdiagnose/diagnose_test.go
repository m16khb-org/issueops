package lintdiagnose

import (
	lintdiagnosecontract "issueops/internal/contract/lintdiagnose"
	"issueops/internal/testsupport"
	"strings"
	"testing"
)

func TestDiagnoseCommandRendersPromptForFailedCommand(t *testing.T) {
	root := t.TempDir()

	result, err := DiagnoseCommand(lintdiagnosecontract.Request{
		RepoRoot:    root,
		CommandArgv: []string{testsupport.WriteFakeCommand(t, t.TempDir(), "lintfail", "#!/bin/sh\necho lint failed >&2\nexit 2\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Failed {
		t.Fatalf("expected failed command result: %+v", result)
	}
	for _, want := range []string{"Execution Failure Output", "lint failed", "diagnosis"} {
		if !strings.Contains(result.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, result.Prompt)
		}
	}
}
