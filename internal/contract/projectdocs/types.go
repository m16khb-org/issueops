// Package projectdocs 패키지는 projectdocs capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package projectdocs

// ReviseRequest 타입은 .issueops 프로젝트 문서 한 개를 통째로 교체하라는 요청이다.
// 기존 파일을 바꿀 때는 ExpectedSHA256으로 읽어 둔 내용과 같은지 확인하고,
// Confirm이 false면 쓰지 않고 dry-run 결과만 돌려준다.
type ReviseRequest struct {
	RepoRoot       string   `json:"repo_root"`
	RelPath        string   `json:"rel_path"`
	Content        string   `json:"content"`
	ExpectedSHA256 string   `json:"expected_sha256,omitempty"`
	Summary        string   `json:"summary"`
	Evidence       []string `json:"evidence,omitempty"`
	Confirm        bool     `json:"confirm"`
}

// ReviseResult 타입은 문서 교체 요청의 결과다. 기존·새 내용의 SHA-256과 바이트 수,
// 실제로 쓰였는지(Confirmed/DryRun)를 담는다.
type ReviseResult struct {
	OK            bool     `json:"ok"`
	Kind          string   `json:"kind"`
	RepoRoot      string   `json:"repo_root"`
	RelPath       string   `json:"rel_path"`
	Path          string   `json:"path"`
	Action        string   `json:"action"`
	Confirmed     bool     `json:"confirmed"`
	DryRun        bool     `json:"dry_run"`
	GeneratedAt   string   `json:"generated_at"`
	CurrentSHA256 string   `json:"current_sha256,omitempty"`
	NextSHA256    string   `json:"next_sha256"`
	Bytes         int      `json:"bytes"`
	Summary       string   `json:"summary"`
	Evidence      []string `json:"evidence,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

// AppendRequest 타입은 cautions 또는 adr 기록 파일 하나를 새로 추가하라는 요청이다.
// Kind가 기록 종류를 정하고, 나머지 필드는 기록 본문을 채운다.
type AppendRequest struct {
	RepoRoot     string   `json:"repo_root"`
	Kind         string   `json:"kind"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Context      string   `json:"context,omitempty"`
	Resolution   string   `json:"resolution,omitempty"`
	Decision     string   `json:"decision,omitempty"`
	Evidence     []string `json:"evidence,omitempty"`
	Alternatives []string `json:"alternatives,omitempty"`
	Consequences string   `json:"consequences,omitempty"`
	Source       string   `json:"source,omitempty"`
}

// AppendResult 타입은 기록 추가 요청의 결과다. 만든 파일의 경로, 추가된 바이트 수,
// SHA-256을 담는다.
type AppendResult struct {
	OK            bool     `json:"ok"`
	Kind          string   `json:"kind"`
	RecordKind    string   `json:"record_kind"`
	RepoRoot      string   `json:"repo_root"`
	RelPath       string   `json:"rel_path"`
	Path          string   `json:"path"`
	GeneratedAt   string   `json:"generated_at"`
	BytesAppended int      `json:"bytes_appended"`
	SHA256        string   `json:"sha256"`
	Warnings      []string `json:"warnings,omitempty"`
}

// ReadResult 타입은 프로젝트 문서 한 개를 읽은 결과다. 파일이 없으면 Exists가 false이고,
// 있으면 Content와 revise 전에 쓸 SHA256을 담는다.
type ReadResult struct {
	OK          bool     `json:"ok"`
	Kind        string   `json:"kind"`
	RepoRoot    string   `json:"repo_root"`
	RelPath     string   `json:"rel_path"`
	Path        string   `json:"path"`
	Exists      bool     `json:"exists"`
	Content     string   `json:"content,omitempty"`
	SHA256      string   `json:"sha256,omitempty"`
	GeneratedAt string   `json:"generated_at"`
	Warnings    []string `json:"warnings,omitempty"`
}

// RouteResult 타입은 주어진 작업(Task)에 앞서 읽어야 할 프로젝트 문서 목록을 담은 결과다.
type RouteResult struct {
	OK          bool                   `json:"ok"`
	Kind        string                 `json:"kind"`
	RepoRoot    string                 `json:"repo_root"`
	Task        string                 `json:"task"`
	GeneratedAt string                 `json:"generated_at"`
	Docs        []ProjectDocRouteEntry `json:"docs"`
	Warnings    []string               `json:"warnings,omitempty"`
}

// ProjectDocRouteEntry 타입은 RouteResult에 포함되는 문서 한 건이다. Reason은 이 문서가
// 선택된 이유이고 Exists는 파일이 실제로 있는지 나타낸다.
type ProjectDocRouteEntry struct {
	RelPath string `json:"rel_path"`
	Path    string `json:"path"`
	Reason  string `json:"reason"`
	Exists  bool   `json:"exists"`
}
