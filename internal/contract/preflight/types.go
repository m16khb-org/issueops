// Package preflight 패키지는 preflight capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package preflight

// Result 타입은 읽기 전용 git preflight 결과다. OK가 false이면 Error와 Path, Detail이
// 실패 원인을 설명하고, 그 외 필드는 저장소 상태 관측값이다. Upstream, Ahead,
// Behind는 upstream이 없으면 null이다.
type Result struct {
	OK               bool           `json:"ok"`
	Error            string         `json:"error,omitempty"`
	Path             string         `json:"path,omitempty"`
	Detail           string         `json:"detail,omitempty"`
	RepoRoot         string         `json:"repo_root,omitempty"`
	Branch           string         `json:"branch,omitempty"`
	Head             string         `json:"head,omitempty"`
	Upstream         *string        `json:"upstream"`
	Ahead            *int           `json:"ahead"`
	Behind           *int           `json:"behind"`
	IsClean          bool           `json:"is_clean"`
	StatusLines      []string       `json:"status_lines"`
	Remotes          []RemoteInfo   `json:"remotes"`
	LastCommit       string         `json:"last_commit"`
	RecentCommits    []CommitInfo   `json:"recent_commits"`
	CommitStyleHints map[string]any `json:"commit_style_hints"`
	StagedFiles      []string       `json:"staged_files"`
	UnstagedFiles    []string       `json:"unstaged_files"`
	UntrackedFiles   []string       `json:"untracked_files"`
	SecretLikePaths  []string       `json:"secret_like_paths"`
	Warnings         []string       `json:"warnings"`
}

// RemoteInfo 타입은 git remote 하나의 이름과 URL이다.
type RemoteInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// CommitInfo 타입은 최근 커밋 하나의 SHA와 제목이다.
type CommitInfo struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
}
