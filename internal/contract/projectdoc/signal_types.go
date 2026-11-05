package projectdoc

// ProjectSignals 타입은 저장소 스캔이 모은 문서 생성 근거다. 파일 목록, 언어,
// 프로필, 테스트·빌드·린트 명령, 기존 에이전트 문서, GitHub workflow, 감지한
// 관례를 담는다.
type ProjectSignals struct {
	Files []string `json:"files"`
	// FilesTotal is set when a result lists only the first files of a larger
	// scan; the plan echoes evidence, not the whole tree.
	FilesTotal          int               `json:"files_total,omitempty"`
	Languages           []string          `json:"languages"`
	PackageManagers     []string          `json:"package_managers"`
	Profile             ProjectProfile    `json:"profile"`
	TestCommands        []EvidenceCommand `json:"test_commands"`
	BuildCommands       []EvidenceCommand `json:"build_commands"`
	LintCommands        []EvidenceCommand `json:"lint_commands"`
	ExistingAgentDocs   []string          `json:"existing_agent_docs"`
	GitHubWorkflows     []string          `json:"github_workflows"`
	DetectedConventions []string          `json:"detected_conventions"`
}

// EvidenceCommand 타입은 근거와 함께 추정한 명령 한 건이다. Evidence는 그
// 명령을 뒷받침하는 파일이고 Confidence는 추정의 신뢰 수준이다.
type EvidenceCommand struct {
	Command    string   `json:"command"`
	Evidence   []string `json:"evidence"`
	Confidence string   `json:"confidence"`
}

// ProjectDocsPlannedFile 타입은 프로젝트 문서 생성 계획의 파일 한 건이다.
// Action은 계획한 작업, Preserved는 기존 내용을 유지하는지 여부다.
type ProjectDocsPlannedFile struct {
	RelPath   string `json:"rel_path"`
	Path      string `json:"path"`
	Action    string `json:"action"`
	Bytes     int    `json:"bytes"`
	SHA256    string `json:"sha256"`
	Reason    string `json:"reason"`
	Preserved bool   `json:"preserved"`
}
