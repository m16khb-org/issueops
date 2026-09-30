package issueopscleanup

import (
	"context"
	"errors"
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestFinishEvidenceUsesSupersedingArtifactBaseBranch(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-finish", Repo: "/repo", IssueURL: "https://github.com/acme/repo/issues/1", RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "https://github.com/acme/repo/pull/2", Provider: "github"}}
	replacement := "https://github.com/acme/repo/pull/3"
	observed := []string{}
	reader := FinishEvidenceReader{
		VerifyMergedHead: func(artifact model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
			observed = append(observed, artifact.URL)
			if artifact.URL == record.RemoteArtifact.URL {
				return model.CleanupRemoteBranchArtifactHead{}, errors.New("original unmerged")
			}
			if artifact.URL != replacement {
				t.Fatalf("unexpected artifact: %+v", artifact)
			}
			return model.CleanupRemoteBranchArtifactHead{BaseRefName: "main"}, nil
		},
		ReadIssueSnapshot: func(ctx context.Context, _ port.IssueProvider, req port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
			if ctx.Value(finishTestContextKey{}) != true || req.Repo != record.Repo || req.URL != record.IssueURL {
				t.Fatal("readback lost snapshot identity or context")
			}
			return port.ExecutionIssueSnapshot{URL: req.URL, Body: model.IssueBodyCompletionStartMarker, State: "closed"}, nil
		},
	}
	req, err := reader.Observe(context.WithValue(context.Background(), finishTestContextKey{}, true), record, model.CleanupFinishRequest{ID: record.ID, SupersededBy: replacement})
	if err != nil || req.Merged || req.MergedBaseBranch != "main" || req.SupersededBy != replacement {
		t.Fatalf("evidence=%+v err=%v", req, err)
	}
	if !reflect.DeepEqual(observed, []string{record.RemoteArtifact.URL, replacement}) {
		t.Fatalf("observations=%v", observed)
	}
}

func TestFinishEvidenceRefusesUnknownMergeBeforeIssueReadback(t *testing.T) {
	reader := FinishEvidenceReader{
		VerifyMergedHead: func(model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
			return model.CleanupRemoteBranchArtifactHead{}, errors.New("provider unavailable")
		},
		ReadIssueSnapshot: func(context.Context, port.IssueProvider, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
			t.Fatal("read issue after failed merge evidence")
			return port.ExecutionIssueSnapshot{}, nil
		},
	}
	_, err := reader.Observe(context.Background(), model.IssueOpsRecord{RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "original"}}, model.CleanupFinishRequest{})
	if err == nil {
		t.Fatal("unknown merge allowed cleanup")
	}
}
