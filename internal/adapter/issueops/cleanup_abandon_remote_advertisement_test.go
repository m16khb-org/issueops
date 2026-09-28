package issueops

import (
	"context"
	"strings"
	"testing"
)

func TestAbandonRemoteAdvertisementMustIdentifyExactBranch(t *testing.T) {
	for _, output := range []string{"oid", "oid\trefs/heads/other", "oid\trefs/heads/106-abandon\nother\trefs/heads/106-abandon"} {
		t.Run(output, func(t *testing.T) {
			state, record := remoteAbandonRecord(t)
			before, err := (CleanupRecordStore{StateRoot: state}).Load(context.Background(), record.ID)
			if err != nil {
				t.Fatal(err)
			}
			deps := remoteAbandonDeps(&remoteAbandonGit{}, &fakeAbandonRemote{})
			deps.Git = func(_ string, args ...string) (int, string) {
				if len(args) > 0 && args[0] == "ls-remote" {
					return 0, output
				}
				if len(args) > 0 && args[0] == "rev-parse" {
					return 1, ""
				}
				return 0, ""
			}
			req := abandonRequest(record.ID, false, "")
			req.ArtifactUnmerged = true
			req.DeleteRemoteBranch = true
			got, err := CleanupAbandon(context.Background(), state, req, deps)
			if err == nil || got.Fingerprint != "" || !strings.Contains(strings.Join(got.Missing, ","), "remote_branch_readable") {
				t.Fatalf("malformed advertisement authorized: result=%+v err=%v", got, err)
			}
			after, err := (CleanupRecordStore{StateRoot: state}).Load(context.Background(), record.ID)
			if err != nil || before.Revision != after.Revision {
				t.Fatalf("preview changed record: %v", err)
			}
		})
	}
}
