package omo

import (
	"path/filepath"

	"issueops/internal/port"
)

type Installer struct{ lifecycleExtension func(string) string }

func NewInstaller(lifecycleExtension func(string) string) Installer {
	return Installer{lifecycleExtension: lifecycleExtension}
}

func (Installer) Name() string { return "omo" }

func (installer Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	plan := NewInstallPlan("omo", req.DryRun)
	_, links, messages, skillErrs := PlanHostSkillLinks(
		req.Root,
		filepath.Join(req.Home, ".omo", "agent", "skills"),
		req.SkillNames,
		"omo",
		req.DryRun,
	)
	plan.Messages(messages)
	plan.Links(links)
	plan.Errs(skillErrs)

	omoRoot := filepath.Join(req.Home, ".omo")
	plan.File(writeOmoUserMCP(filepath.Join(omoRoot, "mcp.json"), req))
	plan.File(WriteTextPlan(
		filepath.Join(omoRoot, "extensions", "issueops.js"),
		"omo_user_lifecycle_extension",
		installer.lifecycleExtension(req.BinPath),
		0o644,
		req.DryRun,
	))
	plan.File(writeOmoProjectMCP(
		filepath.Join(req.Root, "configs", "omo", "mcp.json"),
		"omo_project_mcp_template",
		req.DryRun,
	))
	plan.File(WriteTextPlan(
		filepath.Join(req.Root, "configs", "omo", "issueops.js"),
		"omo_lifecycle_extension_template",
		installer.lifecycleExtension("./bin/issueops"),
		0o644,
		req.DryRun,
	))

	if req.ProjectLocal {
		plan.File(writeOmoProjectMCP(
			filepath.Join(req.Root, ".omo", "mcp.json"),
			"omo_project_mcp_config",
			req.DryRun,
		))
	}

	if req.DryRun {
		plan.Message("dry-run: planned Omo native skills, MCP config, and lifecycle extension without writing")
	}
	return plan.Finish()
}
