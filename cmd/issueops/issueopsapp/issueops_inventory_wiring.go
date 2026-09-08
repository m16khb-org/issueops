package issueopsapp

import (
	issueopsinventoryinbound "issueops/internal/adapter/inbound/issueopsinventory"
	issueopsinventoryoutbound "issueops/internal/adapter/outbound/issueopsinventory"
	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsinventoryapplication "issueops/internal/application/issueopsinventory"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

func issueOpsInventoryListHandler(
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
		issueopsinventoryoutbound.CleanPath{},
	)
	return issueopsinventoryinbound.NewListHandler(service)
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
