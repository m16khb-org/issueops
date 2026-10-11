package commandpolicy

import (
	"encoding/json"
	"path/filepath"
	"slices"

	policy "issueops/internal/contract/policy"
)

type commandPolicyValidationCheck struct {
	label    string
	name     string
	args     []string
	validate func(stdout string) []string
}

func commandPolicyChecks(binary, tempWorkspace, outside string) []commandPolicyValidationCheck {
	return []commandPolicyValidationCheck{
		{
			label: "policy allow",
			name:  binary,
			args:  []string{"policy", "check", "--json", "--workspace-root", tempWorkspace, "--cwd", tempWorkspace, "--", "git", "status", "--short"},
			validate: func(stdout string) []string {
				var allowedEval policy.CommandPolicyEvaluation
				if err := json.Unmarshal([]byte(stdout), &allowedEval); err != nil {
					return []string{err.Error()}
				}
				if !allowedEval.OK || !allowedEval.Allowed {
					return []string{"read-only git status was not allowed"}
				}
				return nil
			},
		},
		{
			label:    "policy deny outside",
			name:     binary,
			args:     []string{"policy", "check", "--json", "--workspace-root", tempWorkspace, "--cwd", outside, "--", "git", "status", "--short"},
			validate: denyValidator("cwd_outside_workspace", "outside cwd was not denied"),
		},
		{
			label:    "policy deny outside path arg",
			name:     binary,
			args:     []string{"policy", "check", "--json", "--workspace-root", tempWorkspace, "--cwd", tempWorkspace, "--", "cat", filepath.Join(outside, "note.txt")},
			validate: denyValidator("path_outside_workspace", "outside path arg was not denied"),
		},
		{
			label:    "policy deny shell",
			name:     binary,
			args:     []string{"policy", "check", "--json", "--workspace-root", tempWorkspace, "--cwd", tempWorkspace, "--", "sh", "-c", "echo ok"},
			validate: denyValidator("shell_interpreter_not_allowed", "shell command was not denied"),
		},
		{
			label: "policy fake-run",
			name:  binary,
			args:  []string{"policy", "fake-run", "--json", "--workspace-root", tempWorkspace, "--cwd", tempWorkspace, "--write", "--", "touch", "marker"},
			validate: func(stdout string) []string {
				var fakeResult policy.CommandFakeRunResult
				if err := json.Unmarshal([]byte(stdout), &fakeResult); err != nil {
					return []string{err.Error()}
				}
				if !fakeResult.OK || fakeResult.Executed || !fakeResult.Policy.Allowed {
					return []string{"fake-run did not report accepted non-execution"}
				}
				return nil
			},
		},
	}
}

// denyValidator returns a validator that expects the policy evaluation to be
// denied with the given reason, reporting failureMessage otherwise.
func denyValidator(reason, failureMessage string) func(stdout string) []string {
	return func(stdout string) []string {
		var eval policy.CommandPolicyEvaluation
		if err := json.Unmarshal([]byte(stdout), &eval); err != nil {
			return []string{err.Error()}
		}
		if eval.Allowed || !slices.Contains(eval.DenyReasons, reason) {
			return []string{failureMessage}
		}
		return nil
	}
}
