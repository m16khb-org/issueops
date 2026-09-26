package apidoc

import (
	reviewfilesppdeps "issueops/internal/adapter/outbound/apidoc/reviewfiles"
	preflightadapter "issueops/internal/adapter/preflight"
)

// production wiring과 같은 실행기를 설치한다. 이 package가 실제로 의존하는
// 대상만 채운다.
func init() {
	reviewfilesppdeps.GitCmd = preflightadapter.GitCmd
	ConfigureReviewFiles(ReviewFileEffects{
		ExtraPrompt: reviewfilesppdeps.ExtraPrompt,
		Diff:        reviewfilesppdeps.Diff,
		Input:       reviewfilesppdeps.Input,
		FullContent: reviewfilesppdeps.FullContent,
		Staged:      reviewfilesppdeps.Staged,
		Tracked:     reviewfilesppdeps.Tracked,
		Normalize:   reviewfilesppdeps.Normalize,
		Evidence:    reviewfilesppdeps.Evidence,
	})
}
