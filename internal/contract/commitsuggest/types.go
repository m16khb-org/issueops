// Package commitsuggest 패키지는 commitsuggest capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package commitsuggest

// Request 타입은 commit message 제안을 구성한다.
type Request struct {
	RepoRoot string `json:"repo_root"`
	Staged   bool   `json:"staged"`
}

// Result 타입은 commit message 제안 결과다. Executed는 실제 커밋 실행 여부이며,
// CommitMessage와 Prompt는 제안된 메시지와 호스트 에이전트에 넘길 프롬프트다.
type Result struct {
	OK            bool   `json:"ok"`
	Executed      bool   `json:"executed"`
	RepoRoot      string `json:"repo_root"`
	Staged        bool   `json:"staged"`
	CommitMessage string `json:"commit_message,omitempty"`
	Prompt        string `json:"prompt,omitempty"`
}
