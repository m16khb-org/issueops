// Package guard 패키지는 guard capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package guard

// CheckRequest 타입은 guard 점검 입력이다. RepoRoot 아래에서 Staged, All, Files 중
// 하나로 점검 대상을 고른다.
type CheckRequest struct {
	RepoRoot string   `json:"repo_root"`
	Staged   bool     `json:"staged"`
	All      bool     `json:"all"`
	Files    []string `json:"files,omitempty"`
}

// CheckResult 타입은 guard 점검 결과다. OK는 차단 finding이 없을 때만 참이고,
// Summary는 심각도별 개수를 센다.
type CheckResult struct {
	OK           bool      `json:"ok"`
	RepoRoot     string    `json:"repo_root"`
	Mode         string    `json:"mode"`
	CheckedFiles []string  `json:"checked_files"`
	Findings     []Finding `json:"findings"`
	Summary      Summary   `json:"summary"`
	Warnings     []string  `json:"warnings,omitempty"`
}

// Finding 타입은 guard 규칙 하나가 낸 지적이다. Rule로 규칙을 구분하고
// File·Line은 알 수 있을 때만 채운다.
type Finding struct {
	Severity    string   `json:"severity"`
	Rule        string   `json:"rule"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Message     string   `json:"message"`
	Evidence    string   `json:"evidence,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// Summary 타입은 점검 결과의 finding 수를 심각도(block, warn, review, info)별로 센다.
type Summary struct {
	Block  int `json:"block"`
	Warn   int `json:"warn"`
	Review int `json:"review"`
	Info   int `json:"info"`
}
