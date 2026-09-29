package cli

import "testing"

func TestLifecycleCommandAdmissionAndUsageKeys(t *testing.T) {
	for _, tc := range []struct {
		name    string
		allowed bool
	}{{"start", true}, {"child", true}, {"execution", true}, {"", false}, {"START", false}, {"start ", false}, {"inspect", false}, {"--help", false}} {
		if got := IsLifecycleCommand(tc.name); got != tc.allowed {
			t.Errorf("IsLifecycleCommand(%q)=%v", tc.name, got)
		}
	}
	for line, want := range map[string]string{
		"  issueops child start --parent ID": "child start",
		"issueops list [--repo PATH]":        "list",
		"issueops status (--id ID)":          "status",
		"issueops execution resume --id ID":  "execution resume",
		"issueops inspect --json":            "",
		"not-issueops child start":           "",
	} {
		if got := IssueOpsUsageKey(line); got != want {
			t.Errorf("IssueOpsUsageKey(%q)=%q want %q", line, got, want)
		}
	}
}
