package issueopsapp

import (
	issueopsinventoryinbound "issueops/internal/adapter/inbound/issueopsinventory"
	issueopscore "issueops/internal/adapter/issueops"
	issueopsinventoryoutbound "issueops/internal/adapter/outbound/issueopsinventory"
	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsinventoryapplication "issueops/internal/application/issueopsinventory"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

func issueOpsInventoryListHandler(
	observers ...issueopsrecord.Observer,
) func(
	string,
	string,
) (issueopsinventorycontract.ListResult, error) {
	return issueOpsInventoryListHandlerWithPaths(issueopsinventoryoutbound.CleanPath{}, observers...)
}

func issueOpsInventoryListHandlerWithPaths(
	paths issueopsinventoryapplication.PathNormalizer,
	observers ...issueopsrecord.Observer,
) func(
	string,
	string,
) (issueopsinventorycontract.ListResult, error) {
	service := issueopsinventoryapplication.NewService(
		issueopsinventoryoutbound.Repository{
			Store: issueOpsRecordStore("inventory", observers...),
		},
		issueopsinventoryoutbound.SystemClock{},
		paths,
	)
	return issueopsinventoryinbound.NewListHandler(service)
}

// callPathNormalizer answers Normalize and Canonical once per path for one
// request. Normalize runs git to find the main worktree, and one `next` call
// asks it for the cwd twice (the source root and the inventory filter). The
// memo lives only for that call, so a long-running MCP service never serves a
// stale repository identity.
type callPathNormalizer struct {
	base       issueopsinventoryapplication.PathNormalizer
	normalized map[string]string
	canonical  map[string]string
}

func newCallPathNormalizer(base issueopsinventoryapplication.PathNormalizer) *callPathNormalizer {
	return &callPathNormalizer{base: base, normalized: map[string]string{}, canonical: map[string]string{}}
}

func (paths *callPathNormalizer) Normalize(path string) string {
	if hit, ok := paths.normalized[path]; ok {
		return hit
	}
	resolved := paths.base.Normalize(path)
	paths.normalized[path] = resolved
	return resolved
}

func (paths *callPathNormalizer) Canonical(path string) string {
	if hit, ok := paths.canonical[path]; ok {
		return hit
	}
	resolved := paths.base.Canonical(path)
	paths.canonical[path] = resolved
	return resolved
}

// issueOpsCycleIDLister는 저장소 필터를 inventory list에 위임하고 ID만 돌려준다.
// review-metrics는 worktree common-dir 해석을 다시 구현하지 않는다.
func issueOpsCycleIDLister(observers ...issueopsrecord.Observer) func(string, string) ([]string, []string, error) {
	list := issueOpsInventoryListHandler(observers...)
	return func(stateRoot, repo string) ([]string, []string, error) {
		result, err := list(stateRoot, repo)
		if err != nil {
			return nil, nil, err
		}
		ids := make([]string, 0, len(result.Entries))
		for _, entry := range result.Entries {
			ids = append(ids, entry.ID)
		}
		// 디코딩 실패 record는 Entries에 들어오지 않는다. 그 사실을 버리면
		// 집계가 조용히 좁아지므로 unreadable ID를 그대로 넘긴다.
		return ids, append([]string(nil), result.UnreadableIDs...), nil
	}
}

// issueOpsCycleOutcomeLister filters retained cycle outcomes with the same path
// normalization inventory list applies to live records, so --repo selects the
// same repository for both populations.
func issueOpsCycleOutcomeLister(stateRoot, repo string) ([]issueopscontract.IssueOpsCycleOutcome, []string, error) {
	outcomes, unreadable, err := issueopscore.ListCycleOutcomes(stateRoot)
	if err != nil || repo == "" {
		return outcomes, unreadable, err
	}
	paths := issueopsinventoryoutbound.CleanPath{}
	want := paths.Normalize(repo)
	matched := outcomes[:0]
	for _, outcome := range outcomes {
		if paths.Normalize(outcome.Repo) == want {
			matched = append(matched, outcome)
		}
	}
	return matched, unreadable, nil
}
