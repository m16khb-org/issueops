package issueopscli

import (
	apidoccli "issueops/cmd/issueops/apidoc"
	mcpclippdeps "issueops/cmd/issueops/mcpcli"
	auditppdeps "issueops/internal/adapter/audit"
	issueopsppdeps "issueops/internal/adapter/issueops"
	implementationppdeps "issueops/internal/adapter/issueops/implementation"
	reviewfilesppdeps "issueops/internal/adapter/outbound/apidoc/reviewfiles"
	policyadapter "issueops/internal/adapter/policy"
	preflightadapter "issueops/internal/adapter/preflight"
)

// production wiring과 같은 실행기를 설치한다. 이 package가 실제로 의존하는
// 대상만 채운다.
func init() {
	auditppdeps.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	implementationppdeps.GitCmd = preflightadapter.GitCmd
	implementationppdeps.GitCmdRaw = preflightadapter.GitCmdRaw
	issueopsppdeps.GitCmd = preflightadapter.GitCmd
	issueopsppdeps.GitCmdRaw = preflightadapter.GitCmdRaw
	issueopsppdeps.GitOut = preflightadapter.GitOut
	mcpclippdeps.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	mcpclippdeps.FakeRunCommand = policyadapter.FakeRunCommand
	reviewfilesppdeps.GitCmd = preflightadapter.GitCmd
	apidoccli.ConfigureReviewFiles(apidoccli.ReviewFileEffects{
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
