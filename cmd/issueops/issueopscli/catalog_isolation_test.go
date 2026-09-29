package issueopscli

import "testing"

func TestCLIHelpUsesEachInvocationsCatalog(t *testing.T) {
	first, second := testCLIUsageDependencies(), testCLIUsageDependencies()
	first.Usage += "\nfirst invocation\n"
	second.Usage += "\nsecond invocation\n"
	first.ChildUsage += "\nfirst child"
	second.ChildUsage += "\nsecond child"
	for _, deps := range []Dependencies{first, second, first} {
		output, err := captureProjectCLIStderr(t, func() error { return RunIssueOpsWithDependencies([]string{"--help"}, deps) })
		if err != nil || output != deps.Usage {
			t.Fatalf("lifecycle help ignores invocation catalog: err=%v output=%q", err, output)
		}
		child, err := captureStdoutAndErrorForIssueOps(t, func() error { return RunIssueOpsWithDependencies([]string{"child", "--help"}, deps) })
		if err != nil || child != deps.ChildUsage+"\n" {
			t.Fatalf("child help ignores invocation catalog: err=%v output=%q", err, child)
		}
	}
}
