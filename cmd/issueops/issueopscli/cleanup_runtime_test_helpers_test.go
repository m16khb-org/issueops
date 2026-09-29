package issueopscli

import (
	"context"
	"errors"
	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	"issueops/cmd/issueops/issueopscli/remoteverify"
	orcaadapter "issueops/internal/adapter/orca"
	provideradapter "issueops/internal/adapter/provider"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
	"strings"
)

func testCleanupRuntime() feedbackcleanup.Deps {
	orphans := orphanCleaner()
	orca := orcaadapter.New()
	execution := orcaadapter.NewExecution()
	return feedbackcleanup.Deps{
		RemoveOrcaWorktree: func(ctx context.Context, id string) error {
			return normalizeOrcaRemoveWorktreeErr(orca.RemoveWorktree(ctx, id, false))
		},
		OrcaIntent: execution, OrcaOwner: execution,
		VerifyMerged: func(a model.IssueOpsRemoteArtifactVerification) error {
			return remoteverify.VerifyRemoteArtifactMergedLive(context.Background(), a)
		},
		VerifyMergedHead: func(a model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
			return remoteverify.VerifyRemoteArtifactMergedHeadLive(context.Background(), a)
		},
		ObserveArtifactMerged: remoteverify.ObserveRemoteArtifactMergedLive,
		Provider:              provideradapter.Resolve,
		OrphanPreview:         orphans.Preview, OrphanApply: orphans.Apply,
	}
}

// normalizeOrcaRemoveWorktreeErr는 orca 워크트리 회수 오류를 멱등 계약으로
// 정규화한다: "이미 없음"(typed not_found 계열)은 제거 목표가 이미 달성된
// 상태이므로 성공이다(#97 — cleanup finish 재실행 수렴의 전제).
func normalizeOrcaRemoveWorktreeErr(err error) error {
	if err == nil {
		return nil
	}
	if orcaErr, ok := errors.AsType[*port.OrcaError](err); ok && strings.Contains(strings.ToLower(orcaErr.Code), "not_found") {
		return nil
	}
	// 폴백: orca CLI 산문 메시지 매칭. 문구/로캘 변경에 취약하므로
	// 타입드 코드가 항상 우선이다(C2-F5).
	if strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "unknown worktree") {
		return nil
	}
	return err
}
