// Package inspect is the adapter that observes the local harness installation:
// skills, docs, and the Codex, Claude, and other host integration and receipt
// state that the harness inspect result reports.
package inspect

import (
	"issueops/internal/adapter/outbound/osfile"
	inspectcontract "issueops/internal/contract/inspect"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Observer gathers the install and integration facts for the harness inspect
// result. Its function and path fields are seams for effects and tests.
type Observer struct {
	ListDocs func(string) []string
	// Now와 HostVersion은 결정적 테스트를 위한 seam이다. 비어 있으면 time.Now이고
	// 현재 host 버전은 관측 불가로 취급한다.
	Now         func() time.Time
	HostVersion func(host string) string
	// ReceiptRoot가 있으면 receipt 파일은 이 root 안에 있어야 한다.
	ReceiptRoot string
	// OpencodeConfigRoot는 home에서 opencode 설정 디렉터리를 정한다. 설치기와 같은
	// 해석기를 쓰며, 비어 있으면 XDG_CONFIG_HOME 없이 $HOME/.config/opencode다.
	OpencodeConfigRoot func(home string) string
}

// Inspect returns the harness install Info for root: skills, docs, Codex and
// Claude skill and MCP status, and per-host integration. The docs listing runs
// concurrently with the other observations.
func (observer Observer) Inspect(root, target, home, version, skillName string, options inspectcontract.Options) inspectcontract.Info {
	codexSkill := filepath.Join(home, ".codex", "skills", skillName)
	claudeSkill := filepath.Join(home, ".claude", "skills", skillName)
	projectClaudeSkill := filepath.Join(root, ".claude", "skills", skillName)
	mcpBinary := filepath.Join(root, "bin", osfile.ExecutableName("issueops"))
	// The docs listing waits on git; observe the rest of the install meanwhile.
	var (
		docs []string
		wg   sync.WaitGroup
	)
	wg.Go(func() { docs = observer.ListDocs(root) })
	skills := ListSkills(root, skillName)
	hosts := observer.observeHosts(root, home, skillName, options)
	wg.Wait()
	return inspectcontract.Info{
		OK:           true,
		Version:      version,
		IssueOpsRoot: root,
		TargetRepo:   target,
		Skills:       skills,
		Docs:         docs,
		Integration: inspectcontract.IntegrationStatus{
			CodexSkillPath:         codexSkill,
			CodexSkillInstalled:    exists(filepath.Join(codexSkill, "SKILL.md")),
			CodexMCPConfigured:     codexMCPConfigured(filepath.Join(home, ".codex", "config.toml")),
			ClaudeSkillPath:        claudeSkill,
			ClaudeSkillInstalled:   exists(filepath.Join(claudeSkill, "SKILL.md")),
			ProjectClaudeSkillPath: projectClaudeSkill,
			ProjectClaudeSkill:     exists(filepath.Join(projectClaudeSkill, "SKILL.md")),
			ProjectClaudeMCPConfig: exists(filepath.Join(root, ".mcp.json")),
			MCPBinaryPath:          mcpBinary,
			Hosts:                  hosts,
		},
		GeneratedAt: observer.now().UTC().Format(time.RFC3339),
	}
}

// ListSkills returns the skill directories under root/skills sorted by name, with
// whether each has a SKILL.md and an agents/openai.yaml. It returns nil when the
// skills directory cannot be read.
func ListSkills(root, skillName string) []inspectcontract.SkillInfo {
	dir := filepath.Join(root, "skills")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []inspectcontract.SkillInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		s := inspectcontract.SkillInfo{
			Name:       e.Name(),
			Path:       p,
			HasSkillMD: exists(filepath.Join(p, "SKILL.md")),
			HasOpenAI:  exists(filepath.Join(p, "agents", "openai.yaml")),
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func codexMCPConfigured(path string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(b), "[mcp_servers.issueops]")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
