package issueopsapp

import (
	apidoccli "issueops/cmd/issueops/apidoc"
	mcpclideps "issueops/cmd/issueops/mcpcli"
	resourcesdeps "issueops/cmd/issueops/mcpcli/resources"
	policyclideps "issueops/cmd/issueops/policycli"
	statusclideps "issueops/cmd/issueops/statuscli"
	preflightfuzzdeps "issueops/cmd/issueops/validationcli/preflightfuzz"
	auditdeps "issueops/internal/adapter/audit"
	gatesdeps "issueops/internal/adapter/gates"
	gitworktreedeps "issueops/internal/adapter/gitworktree"
	issueopsdeps "issueops/internal/adapter/issueops"
	implementationdeps "issueops/internal/adapter/issueops/implementation"
	reviewfilesdeps "issueops/internal/adapter/outbound/apidoc/reviewfiles"
	policyadapter "issueops/internal/adapter/policy"
	preflightadapter "issueops/internal/adapter/preflight"
	workerdeps "issueops/internal/adapter/worker"
	policyapp "issueops/internal/application/policy"
)

// configurePolicyAndGitObservers는 명령 정책 평가·실행과 git 관측을 설치한다.
//
// 두 기능 모두 프로세스를 띄운다. 어떤 실행기를 쓸지는 composition root의
// 결정이고, 소비자는 요청과 결과 형식만 안다.
func configurePolicyAndGitObservers() {
	configurePolicyAndGitObserversWithLookup(func(workspace string) (string, bool) {
		return newActiveCycleReader(issueopsdeps.IssueOpsStateRoot()).PreparedBaseBranchForWorkspace(workspace)
	})
}

func configurePolicyAndGitObserversWithLookup(lookup policyapp.PreparedBaseBranchLookup) {
	evaluator := policyadapter.NewEvaluator(lookup)
	auditdeps.EvaluateCommandPolicy = evaluator.Evaluate
	gatesdeps.EvaluateCommandPolicy = evaluator.Evaluate
	gatesdeps.RunCommand = evaluator.Run
	gitworktreedeps.GitCmd = preflightadapter.GitCmd
	gitworktreedeps.GitOut = preflightadapter.GitOut
	implementationdeps.GitCmd = preflightadapter.GitCmd
	implementationdeps.GitCmdRaw = preflightadapter.GitCmdRaw
	issueopsdeps.GitCmd = preflightadapter.GitCmd
	issueopsdeps.GitCmdRaw = preflightadapter.GitCmdRaw
	issueopsdeps.GitOut = preflightadapter.GitOut
	mcpclideps.EvaluateCommandPolicy = evaluator.Evaluate
	mcpclideps.FakeRunCommand = evaluator.FakeRun
	policyclideps.EvaluateCommandPolicy = evaluator.Evaluate
	policyclideps.FakeRunCommand = evaluator.FakeRun
	policyclideps.RunReadOnlyCommand = evaluator.RunReadOnly
	preflightfuzzdeps.GitCmd = preflightadapter.GitCmd
	resourcesdeps.CommandPolicySummary = policyadapter.CommandPolicySummary
	reviewfilesdeps.GitCmd = preflightadapter.GitCmd
	apidoccli.ConfigureReviewFiles(apidoccli.ReviewFileEffects{
		ExtraPrompt: reviewfilesdeps.ExtraPrompt,
		Diff:        reviewfilesdeps.Diff,
		Input:       reviewfilesdeps.Input,
		FullContent: reviewfilesdeps.FullContent,
		Staged:      reviewfilesdeps.Staged,
		Tracked:     reviewfilesdeps.Tracked,
		Normalize:   reviewfilesdeps.Normalize,
		Evidence:    reviewfilesdeps.Evidence,
	})
	statusclideps.RunReadOnlyCommand = evaluator.RunReadOnly
	workerdeps.RunReadOnlyCommand = evaluator.RunReadOnly
}
