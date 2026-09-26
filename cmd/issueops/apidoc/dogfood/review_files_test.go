package dogfood

import (
	apidoc "issueops/cmd/issueops/apidoc"
	"issueops/internal/adapter/outbound/apidoc/reviewfiles"
)

func init() {
	apidoc.ConfigureReviewFiles(apidoc.ReviewFileEffects{
		ExtraPrompt: reviewfiles.ExtraPrompt,
		Diff:        reviewfiles.Diff,
		Input:       reviewfiles.Input,
		FullContent: reviewfiles.FullContent,
		Staged:      reviewfiles.Staged,
		Tracked:     reviewfiles.Tracked,
		Normalize:   reviewfiles.Normalize,
		Evidence:    reviewfiles.Evidence,
	})
}
