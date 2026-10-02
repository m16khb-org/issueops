// Package inspect는 inspect capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package inspect

type InspectInfo struct {
	OK           bool              `json:"ok"`
	Version      string            `json:"version"`
	IssueOpsRoot string            `json:"issueops_root"`
	TargetRepo   string            `json:"target_repo"`
	Skills       []SkillInfo       `json:"skills"`
	Docs         []string          `json:"docs"`
	Integration  IntegrationStatus `json:"integration"`
	GeneratedAt  string            `json:"generated_at"`
}

// Options는 inspect 호출마다 달라지는 입력이다. 빈 값은 기본 동작이다.
type Options struct {
	// CodexHome은 Codex 설정 디렉터리다. 비어 있으면 <home>/.codex다.
	CodexHome string
	// HostReceipts는 실제 host 실행 receipt 파일 경로다. 비어 있으면 읽지 않는다.
	HostReceipts string
}

type IntegrationStatus struct {
	CodexSkillPath         string `json:"codex_skill_path"`
	CodexSkillInstalled    bool   `json:"codex_skill_installed"`
	CodexMCPConfigured     bool   `json:"codex_mcp_configured"`
	ClaudeSkillPath        string `json:"claude_skill_path"`
	ClaudeSkillInstalled   bool   `json:"claude_skill_installed"`
	ProjectClaudeSkillPath string `json:"project_claude_skill_path"`
	ProjectClaudeSkill     bool   `json:"project_claude_skill"`
	ProjectClaudeMCPConfig bool   `json:"project_claude_mcp_config"`
	MCPBinaryPath          string `json:"mcp_binary_path"`

	// Hosts는 host별 여섯 관측이다. 파일/파싱으로 확인하는 Installed·Linked·
	// Configured와 host receipt로만 판정하는 Discovered·Connected·Protocol을
	// 구분해 담는다.
	Hosts []HostIntegration `json:"hosts"`
}

// Observation status 값이다. 증거가 없는 상태를 verified로 올리지 않는다.
const (
	ObservationVerified   = "verified"
	ObservationFailed     = "failed"
	ObservationUnknown    = "unknown"
	ObservationNotChecked = "not_checked"
)

// Host 이름이다.
const (
	HostCodex  = "codex"
	HostClaude = "claude"
	HostOmo    = "omo"
)

// HostReceiptsSchemaVersion은 --host-receipts 파일이 지원하는 유일한 schema다.
const HostReceiptsSchemaVersion = 1

// HostIntegration은 한 host의 여섯 관측이다. Installed·Linked·Configured는
// 파일/파싱, Discovered·Connected·Protocol은 host receipt가 있을 때만 판정한다.
type HostIntegration struct {
	Host       string      `json:"host"`
	ConfigPath string      `json:"config_path"`
	Transport  string      `json:"transport"`
	SkillPath  string      `json:"skill_path"`
	Installed  Observation `json:"installed"`
	Linked     Observation `json:"linked"`
	Configured Observation `json:"configured"`
	Discovered Observation `json:"discovered"`
	Connected  Observation `json:"connected"`
	Protocol   Observation `json:"protocol"`
}

// Observation은 증거와 함께 남기는 단일 관측이다. ConfigSHA256은 비밀을 제거한
// issueops entry의 canonical JSON hash이며 receipt의 신선도 검사에 쓴다.
type Observation struct {
	Status             string   `json:"status"`
	Source             string   `json:"source"`
	ObservedAt         string   `json:"observed_at"`
	Reason             string   `json:"reason"`
	HostVersion        string   `json:"host_version"`
	RequestedRevision  string   `json:"requested_revision"`
	NegotiatedRevision string   `json:"negotiated_revision"`
	ConfigSHA256       string   `json:"config_sha256"`
	Features           []string `json:"features"`
}

// HostReceipts는 `inspect --host-receipts FILE`이 읽는 파일 형식이다.
type HostReceipts struct {
	SchemaVersion int               `json:"schema_version"`
	Hosts         []HostIntegration `json:"hosts"`
}

type SkillInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	HasSkillMD  bool   `json:"has_skill_md"`
	HasOpenAI   bool   `json:"has_openai_yaml"`
	Description string `json:"description,omitempty"`
}
