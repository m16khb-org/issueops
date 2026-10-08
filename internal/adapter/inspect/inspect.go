package inspect

import (
	inspectcontract "issueops/internal/contract/inspect"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Observer struct {
	ListDocs func(string) []string
	// Now와 HostVersion은 결정적 테스트를 위한 seam이다. 비어 있으면 time.Now이고
	// 현재 host 버전은 관측 불가로 취급한다.
	Now         func() time.Time
	HostVersion func(host string) string
	// ReceiptRoot가 있으면 receipt 파일은 이 root 안에 있어야 한다.
	ReceiptRoot string
}

func (observer Observer) Inspect(root, target, home, version, skillName string, options inspectcontract.Options) inspectcontract.InspectInfo {
	codexSkill := filepath.Join(home, ".codex", "skills", skillName)
	claudeSkill := filepath.Join(home, ".claude", "skills", skillName)
	projectClaudeSkill := filepath.Join(root, ".claude", "skills", skillName)
	mcpBinary := filepath.Join(root, "bin", "issueops")
	// The docs listing waits on git; observe the rest of the install meanwhile.
	var (
		docs []string
		wg   sync.WaitGroup
	)
	wg.Go(func() { docs = observer.ListDocs(root) })
	skills := ListSkills(root, skillName)
	hosts := observer.observeHosts(root, home, skillName, options)
	wg.Wait()
	return inspectcontract.InspectInfo{
		OK:           true,
		Version:      version,
		IssueOpsRoot: root,
		TargetRepo:   target,
		Skills:       skills,
		Docs:         docs,
		Integration: inspectcontract.IntegrationStatus{
			CodexSkillPath:         codexSkill,
			CodexSkillInstalled:    Exists(filepath.Join(codexSkill, "SKILL.md")),
			CodexMCPConfigured:     CodexMCPConfigured(filepath.Join(home, ".codex", "config.toml")),
			ClaudeSkillPath:        claudeSkill,
			ClaudeSkillInstalled:   Exists(filepath.Join(claudeSkill, "SKILL.md")),
			ProjectClaudeSkillPath: projectClaudeSkill,
			ProjectClaudeSkill:     Exists(filepath.Join(projectClaudeSkill, "SKILL.md")),
			ProjectClaudeMCPConfig: Exists(filepath.Join(root, ".mcp.json")),
			MCPBinaryPath:          mcpBinary,
			Hosts:                  hosts,
		},
		GeneratedAt: observer.now().Format(time.RFC3339),
	}
}

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
			HasSkillMD: Exists(filepath.Join(p, "SKILL.md")),
			HasOpenAI:  Exists(filepath.Join(p, "agents", "openai.yaml")),
		}
		s.Description = readSkillDescription(filepath.Join(p, "SKILL.md"))
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func readSkillDescription(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "description:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "description:")), `"'`)
		}
	}
	return ""
}

func CodexMCPConfigured(path string) bool {
	b, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(b), "[mcp_servers.issueops]")
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
