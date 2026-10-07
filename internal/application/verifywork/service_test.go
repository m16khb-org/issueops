package verifywork

import (
	"errors"
	guard "issueops/internal/contract/guard"
	policy "issueops/internal/contract/policy"
	preflight "issueops/internal/contract/preflight"
	projectdoc "issueops/internal/domain/projectdoc"
	"reflect"
	"testing"
)

func TestServicePreservesCheckOrderAndFailureEvidence(t *testing.T) {
	for _, command := range []bool{false, true} {
		t.Run(map[bool]string{false: "skipped", true: "denied"}[command], func(t *testing.T) {
			calls := []string{}
			service := Service{
				ResolveTarget: func(repo string) string { calls = append(calls, "resolve"); return repo },
				GitStatus: func(root string) (string, error) {
					calls = append(calls, "git")
					return "?? draft\n", errors.New("exit status 128")
				},
				Preflight: func(root string) preflight.PreflightResult {
					calls = append(calls, "preflight")
					return preflight.PreflightResult{OK: false, Path: root}
				},
				Guard: func(req guard.GuardCheckRequest) guard.GuardCheckResult {
					calls = append(calls, "guard")
					if req.RepoRoot != "." || !req.All || req.Staged {
						t.Fatalf("guard request: %+v", req)
					}
					return guard.GuardCheckResult{OK: false, Mode: "all"}
				},
				RunCommand: func(req policy.CommandPolicyRequest) policy.CommandRunResult {
					calls = append(calls, "command")
					if req.WorkspaceRoot != "." || req.CWD != "." || req.Timeout != "30s" || !reflect.DeepEqual(req.Argv, []string{"sh", "-c", "true"}) {
						t.Fatalf("policy request: %+v", req)
					}
					return policy.CommandRunResult{OK: false}
				},
				ProjectSignals: func(root string) projectdoc.ProjectSignals {
					calls = append(calls, "signals")
					return projectdoc.ProjectSignals{}
				},
			}
			var argv []string
			wantCalls := []string{"resolve", "git", "preflight", "guard"}
			wantWarnings := []string{"git status: exit status 128", "git preflight reported issues", "guard check has blocking findings"}
			if command {
				argv = []string{"sh", "-c", "true"}
				wantCalls = append(wantCalls, "command")
				wantWarnings = append(wantWarnings, "read-only verification command failed or was denied")
			}
			wantCalls = append(wantCalls, "signals")
			result := service.Run(".", true, argv)
			if !reflect.DeepEqual(calls, wantCalls) || !reflect.DeepEqual(result.Warnings, wantWarnings) {
				t.Fatalf("calls=%v warnings=%v", calls, result.Warnings)
			}
			if result.OK || result.Repo != "." || result.GitStatus != "?? draft\n" || result.SuggestedCommands == nil || result.EvidenceMatrix == nil {
				t.Fatalf("result: %+v", result)
			}
			last := result.EvidenceMatrix[2]
			if command && (last.OK || last.Status != "failed" || last.Command != "sh -c true") {
				t.Fatalf("denied: %+v", last)
			}
			if !command && (!last.OK || last.Status != "skipped" || result.Command != nil) {
				t.Fatalf("skipped: %+v", last)
			}
		})
	}
}
