package claude

import (
	"path/filepath"

	"issueops/internal/port"
)

type Installer struct{ deps Dependencies }

func NewInstaller(deps Dependencies) Installer { return Installer{deps: deps} }

func (Installer) Name() string { return "claude" }

func (installer Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	plan := installer.deps.NewInstallPlan("claude", req.DryRun)

	_, links, messages, skillErrs := installer.deps.PlanHostSkillLinks(req.Root, filepath.Join(req.Home, ".claude", "skills"), req.SkillNames, "claude", req.DryRun)
	plan.Messages(messages)
	plan.Links(links)
	plan.Errs(skillErrs)

	settingsFile, hookMessages, settingsErr := installer.writeClaudeSettings(filepath.Join(req.Home, ".claude", "settings.json"), req)
	plan.File(settingsFile, settingsErr)
	plan.Messages(hookMessages)
	plan.File(installer.writeClaudeUserMCP(filepath.Join(req.Home, ".claude.json"), req))

	mcpConfig := claudeProjectMCPConfig()
	plan.File(installer.deps.WriteJSONPlan(filepath.Join(req.Root, "configs", "claude", "mcp.project.json"), "claude_project_mcp_template", mcpConfig, 0o644, req.DryRun))

	hooksTemplatePath := filepath.Join(req.Root, "configs", "claude", "hooks.settings.json")
	plan.File(installer.deps.WriteJSONPlan(hooksTemplatePath, "claude_hooks_template", installer.claudeSettingsConfig("./bin/issueops"), 0o644, req.DryRun))

	if req.ProjectLocal && req.MCPTransport == mcpTransportHTTP {
		plan.File(installer.removeClaudeProjectMCP(filepath.Join(req.Root, ".mcp.json"), req.DryRun))
		plan.Message("project-local Claude MCP uses the user-scope issueops HTTP entry; no project entry or secret is written")
	} else if req.ProjectLocal {
		plan.File(installer.deps.WriteJSONPlan(filepath.Join(req.Root, ".mcp.json"), "claude_project_mcp_config", mcpConfig, 0o644, req.DryRun))
	}

	if req.DryRun {
		plan.Message("dry-run: planned Claude user skills, MCP config, and lifecycle hooks without writing")
	}

	return plan.Finish()
}

func claudeProjectMCPConfig() map[string]any {
	return map[string]any{
		"mcpServers": map[string]any{
			"issueops_project": map[string]any{
				"type":    "stdio",
				"command": "./bin/issueops",
				"args":    []string{"mcp"},
				"env": map[string]any{
					"ISSUEOPS_ROOT": ".",
				},
			},
		},
	}
}
