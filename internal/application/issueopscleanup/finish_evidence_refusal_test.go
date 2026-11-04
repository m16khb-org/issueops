package issueopscleanup

import (
	"context"
	"errors"
	"strings"
	"testing"

	executionissue "issueops/internal/contract/executionissue"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func evidenceRecord() model.Record {
	return model.Record{ID: "io-finish", Repo: "/repo", IssueURL: "https://github.com/acme/repo/issues/1", RemoteArtifact: &model.RemoteArtifactVerification{URL: "https://github.com/acme/repo/pull/2", Provider: "github"}}
}

func TestFinishEvidenceRefusesWithoutVerifiableArtifact(t *testing.T) {
	noRead := func(context.Context, port.IssueProvider, executionissue.SnapshotRequest) (executionissue.Snapshot, error) {
		t.Fatal("issue read before merge evidence existed")
		return executionissue.Snapshot{}, nil
	}
	merged := func(model.RemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
		return model.CleanupRemoteBranchArtifactHead{}, nil
	}
	t.Run("record has no artifact", func(t *testing.T) {
		record := evidenceRecord()
		record.RemoteArtifact = nil
		_, err := (FinishEvidenceReader{VerifyMergedHead: merged, ReadIssueSnapshot: noRead}).Observe(t.Context(), record, model.CleanupFinishRequest{})
		if err == nil || !strings.Contains(err.Error(), "requires a verified remote artifact") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("merge verification not wired", func(t *testing.T) {
		_, err := (FinishEvidenceReader{ReadIssueSnapshot: noRead}).Observe(t.Context(), evidenceRecord(), model.CleanupFinishRequest{})
		if err == nil || !strings.Contains(err.Error(), "merge verification is not configured") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("superseding artifact also unverifiable", func(t *testing.T) {
		failure := errors.New("provider unavailable")
		calls := 0
		reader := FinishEvidenceReader{ReadIssueSnapshot: noRead, VerifyMergedHead: func(model.RemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
			calls++
			return model.CleanupRemoteBranchArtifactHead{}, failure
		}}
		_, err := reader.Observe(t.Context(), evidenceRecord(), model.CleanupFinishRequest{SupersededBy: "  https://github.com/acme/repo/pull/3 "})
		if !errors.Is(err, failure) || !strings.Contains(err.Error(), "superseding merge evidence readback failed") || calls != 2 {
			t.Fatalf("err=%v calls=%d", err, calls)
		}
	})
}

func TestFinishEvidenceReportsIssueReadbackFailureAndMergedEvidence(t *testing.T) {
	record := evidenceRecord()
	merged := func(model.RemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
		return model.CleanupRemoteBranchArtifactHead{BaseRefName: "main"}, nil
	}
	failure := errors.New("issue gone")
	_, err := (FinishEvidenceReader{VerifyMergedHead: merged, ReadIssueSnapshot: func(context.Context, port.IssueProvider, executionissue.SnapshotRequest) (executionissue.Snapshot, error) {
		return executionissue.Snapshot{}, failure
	}}).Observe(t.Context(), record, model.CleanupFinishRequest{})
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "issue readback failed") {
		t.Fatalf("err=%v", err)
	}
	req, err := (FinishEvidenceReader{VerifyMergedHead: merged, ReadIssueSnapshot: func(context.Context, port.IssueProvider, executionissue.SnapshotRequest) (executionissue.Snapshot, error) {
		return executionissue.Snapshot{Body: model.IssueBodyCompletionStartMarker, State: "closed"}, nil
	}}).Observe(t.Context(), record, model.CleanupFinishRequest{ID: record.ID})
	if err != nil || !req.Merged || req.MergedBaseBranch != "main" || !req.IssueClosed {
		t.Fatalf("req=%+v err=%v", req, err)
	}
}
