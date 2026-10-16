// 실행이 참조하는 이슈 스냅샷 DTO다. 스냅샷을 읽어오는 쪽은 I/O를 하지만,
// 결과를 필드로 담아 전달하는 쪽은 그 구현을 알 필요가 없다.
package executionissue

type ExecutionIssueSnapshotRequest struct {
	Repo string `json:"repo"`
	URL  string `json:"url"`
}
type ExecutionIssueSnapshot struct {
	URL    string `json:"url"`
	Body   string `json:"body"`
	State  string `json:"state,omitempty"`
	Source string `json:"source,omitempty"`
}

// IssueProviderCreatePullRequestResult reports the outcome.
type IssueProviderCreatePullRequestResult struct {
	OK      bool   `json:"ok"`
	URL     string `json:"url"`
	Number  string `json:"number"`
	Preview string `json:"preview,omitempty"`
}
