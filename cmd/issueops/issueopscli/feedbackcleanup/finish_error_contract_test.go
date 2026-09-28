package feedbackcleanup

import (
	"errors"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestCleanupFinishEvidenceFailurePreservesErrorRendering(t *testing.T) {
	for _, mode := range []string{"merge", "issue"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
			record := cleanupStatusRecord(t, true, true)
			failure := errors.New("remote observation unavailable")
			var results []any
			var rendered []error
			deps := cleanupStatusDeps(&results)
			provider := &cleanupStatusProvider{}
			deps.Provider = func(string) (port.IssueProvider, error) { return provider, nil }
			deps.VerifyMergedHead = func(model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
				if mode == "merge" {
					return model.CleanupRemoteBranchArtifactHead{}, failure
				}
				return model.CleanupRemoteBranchArtifactHead{}, nil
			}
			if mode == "issue" {
				provider.readErr = failure
			}
			deps.PrintError = func(err error) error { rendered = append(rendered, err); return nil }
			err := RunCleanup([]string{"finish", "--id", record.ID, "--preview", "--json"}, deps)
			if !errors.Is(err, failure) || len(rendered) != 1 || len(results) != 0 {
				t.Fatalf("error contract changed: err=%v errors=%v results=%+v", err, rendered, results)
			}
		})
	}
}
