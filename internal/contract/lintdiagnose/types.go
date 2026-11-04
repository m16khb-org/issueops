// Package lintdiagnose 패키지는 lintdiagnose capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package lintdiagnose

// Request 타입은 진단할 lint 명령(CommandArgv)과 실행할 저장소(RepoRoot)를 지정한다.
type Request struct {
	RepoRoot    string   `json:"repo_root"`
	CommandArgv []string `json:"command_argv"`
}

// Result 타입은 lint 명령 실행 결과와 진단이다. 명령이 실패하면 Failed가 true이고,
// 실패 출력으로 만든 Diagnosis와 호스트 에이전트에 넘길 Prompt가 채워진다.
type Result struct {
	OK          bool     `json:"ok"`
	CommandArgv []string `json:"command_argv"`
	ExitCode    int      `json:"exit_code"`
	Failed      bool     `json:"failed"`
	Diagnosis   string   `json:"diagnosis,omitempty"`
	Prompt      string   `json:"prompt,omitempty"`
}
