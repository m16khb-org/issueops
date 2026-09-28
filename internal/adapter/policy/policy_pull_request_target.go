package policy

import (
	"strings"

	"issueops/internal/domain/commandparse"
	policydomain "issueops/internal/domain/policy"
)

// PreparedBaseBranchLookup observes the active cycle's prepared parent branch.
type PreparedBaseBranchLookup func(workspace string) (string, bool)

// pullRequestTargetDeny는 PR/MR 생성 명령이 진행 중인 사이클의 부모 작업
// 브랜치를 타겟하지 않을 때 거부 사유와 기대값을 돌려준다.
//
// `issueops branch prepare`가 자식 사이클의 base_branch를 우산 브랜치로
// 고정하므로, 그 사이클에서 열리는 MR의 타겟은 그 값 하나뿐이다. 원격 쓰기는
// 되돌리기 번거로운 바깥 작용이라 사후 검증(target_branch_match)만으로는
// 늦다. 잘못된 타겟의 MR이 이미 열린 뒤에야 걸린다.
//
// 진행 중인 사이클이 없으면 판정하지 않는다. IssueOps 밖에서 여는 일상적인
// PR/MR까지 막으면 가드가 아니라 방해가 된다.
func pullRequestTargetDeny(workspaceRoot, cwd string, argv []string, lookup PreparedBaseBranchLookup) (reason, expected string) {
	if lookup == nil {
		return "", ""
	}
	parsed, ok := commandparse.ParseRemotePullRequestCreate(argv)
	if !ok {
		return "", ""
	}
	lookupPath := strings.TrimSpace(cwd)
	if lookupPath == "" {
		lookupPath = strings.TrimSpace(workspaceRoot)
	}
	base, ok := lookup(lookupPath)
	if !ok {
		return "", ""
	}
	return policydomain.DecidePullRequestTarget(policydomain.PullRequestTargetFacts{
		ExpectedBase: base, RequestedBase: parsed.BaseBranch, HasBaseFlag: parsed.HasBaseFlag,
	}), base
}
