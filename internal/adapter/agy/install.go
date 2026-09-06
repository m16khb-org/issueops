package agy

import (
	"path/filepath"

	"issueops/internal/port"
)

type Installer struct{}

func NewInstaller() Installer { return Installer{} }

func (Installer) Name() string { return "agy" }

func (Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	plan := NewInstallPlan("agy", req.DryRun)
	geminiRoot := filepath.Join(req.Home, ".gemini", "config")
	_, links, messages, skillErrs := PlanHostSkillLinks(
		req.Root,
		filepath.Join(geminiRoot, "skills"),
		req.SkillNames,
		"agy",
		req.DryRun,
	)
	plan.Messages(messages)
	plan.Links(links)
	plan.Errs(skillErrs)

	plan.File(writeAgyUserMCP(filepath.Join(geminiRoot, "mcp_config.json"), req))
	plan.File(writeAgyProjectMCP(
		filepath.Join(req.Root, "configs", "agy", "mcp_config.json"),
		"agy_project_mcp_template",
		req.DryRun,
	))

	if req.ProjectLocal {
		plan.File(writeAgyProjectMCP(
			filepath.Join(req.Root, ".agents", "mcp_config.json"),
			"agy_project_mcp_config",
			req.DryRun,
		))
	}

	if req.DryRun {
		plan.Message("dry-run: planned agy native skills and MCP config without writing")
	}
	return plan.Finish()
}
