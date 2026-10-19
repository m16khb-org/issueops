package issueops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
)

// 봉인 digest(어댑터)와 리뷰가 기록하는 digest(domain)는 같은 문서에서 같은 값이어야
// 승인이 실행 뒤에도 stale로 오판되지 않는다.
func TestReviewArchitectureSourceSealedDigestMatchesRecordedDigest(t *testing.T) {
	root := t.TempDir()
	record := issueops.IssueOpsRecord{
		ID: "io-arch",
		Execution: &issueops.Execution{Workspace: issueops.Workspace{
			Root: root, ArtifactDir: ".issueops/issues/1/artifact",
		}},
	}
	source := ReviewArchitectureSource{}

	if body, err := source.Document("", record); err != nil || body != "" {
		t.Fatalf("a missing sealed file reads as an empty document, got %q err=%v", body, err)
	}
	if _, err := source.SealedDigest(record); err == nil {
		t.Fatal("a missing sealed architecture must not produce a digest")
	}

	body := strings.Join(issueopsdomain.RequiredArchitectureSections, "\n본문\n") + "\n본문\n"
	path := filepath.Join(root, ".issueops/issues/1/artifact/architecture.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	recorded, err := reviewdomain.ArchitectureDocumentDigest(body)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := source.SealedDigest(record)
	if err != nil || sealed != recorded {
		t.Fatalf("sealed digest %q (err %v) must equal the recorded digest %q", sealed, err, recorded)
	}

	record.Execution.Workspace.ArtifactDir = ""
	if _, err := source.SealedDigest(record); err == nil {
		t.Fatal("a record without a sealed directory must not read a file from the worktree root")
	}
}

// execution prepare가 봉인하는 파일의 digest는 리뷰를 기록할 때 묶은 digest와 같아야 한다.
// 같지 않으면 봉인 직후 implement 진입이 architecture_review_stale로 막힌다.
func TestSealedArchitectureMatchesTheDigestBoundAtReview(t *testing.T) {
	t.Parallel()

	stateRoot, record := executionPrepareRecord(t)
	record.Intent = issueOpsIntentContractForTest()
	record.ArchitectureReview = nil
	if _, err := writeIssueOps(context.Background(), stateRoot, record); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordIssueOpsArchitectureReview(stateRoot, record.ID, reviewcontract.ArchitectureReviewRequest{
		ProblemSummary: "p", ProposedArchitecture: "a", RefactorPlan: "r", Alternatives: []string{"x"}, Risks: []string{"y"},
		Verification: []string{"architecture review checked alternatives and risks"}, Approved: true,
	}); err != nil {
		t.Fatal(err)
	}
	reviewed, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil || reviewed.ArchitectureReview == nil || reviewed.ArchitectureReview.ReviewedArchitectureDigest == "" {
		t.Fatalf("review must bind a digest: %+v err=%v", reviewed.ArchitectureReview, err)
	}
	root := t.TempDir()
	reviewed.WorktreePath = root
	reviewed.Execution = preparedExecution(issueops.LeaseStatusReleased)
	reviewed.Execution.Workspace.SourceRoot = reviewed.Repo
	reviewed.Execution.Workspace.Root = root
	reviewed.Execution.Workspace.Branch = reviewed.Branch
	reviewed.Execution.Workspace.BaseHead = reviewed.BranchPrepare.BaseSHA
	reviewed.Execution.Workspace.LinkedAt = "2026-08-27T00:00:00Z"
	reviewed.Execution.Workspace.ArtifactDir = ".issueops/issues/480/artifact"
	if _, err := materializeStagedArtifacts(stateRoot, reviewed); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	sealed, err := ReviewArchitectureSource{}.SealedDigest(reviewed)
	if err != nil || sealed != reviewed.ArchitectureReview.ReviewedArchitectureDigest {
		t.Fatalf("sealed digest %q (err %v) must equal the digest bound at review %q", sealed, err, reviewed.ArchitectureReview.ReviewedArchitectureDigest)
	}
}
