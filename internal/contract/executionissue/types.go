// Package executionissue 패키지는 실행이 참조하는 이슈 스냅샷 DTO를 소유한다.
// 스냅샷을 읽어오는 쪽은 I/O를 하지만, 결과를 필드로 담아 전달하는 쪽은 그 구현을 알 필요가 없다.
package executionissue

// SnapshotRequest 타입은 이슈 스냅샷을 읽을 저장소(Repo)와 이슈 URL을 지정한다.
type SnapshotRequest struct {
	Repo string `json:"repo"`
	URL  string `json:"url"`
}

// Snapshot 타입은 읽어 온 이슈의 URL, 본문, 상태와 출처를 담는다.
type Snapshot struct {
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
