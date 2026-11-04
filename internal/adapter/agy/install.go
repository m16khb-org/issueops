package agy

import (
	"path/filepath"

	"issueops/internal/adapter/outbound/osfile"
	"issueops/internal/port"
)

// Installer installs issueops into the agy host.
type Installer struct{ deps Dependencies }

// NewInstaller returns an Installer that uses deps for planning and file
// writes.
func NewInstaller(deps Dependencies) Installer { return Installer{deps: deps} }

var _ port.HostInstaller = Installer{}

// Name returns the host name, "agy".
func (Installer) Name() string { return "agy" }

// Install plans and applies the agy native skill links and MCP configs: the user
// config, the project template, and, with req.ProjectLocal, the project-local
// config. With req.DryRun it only plans and reports.
func (installer Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	plan := installer.deps.NewInstallPlan("agy", req.DryRun)
	geminiRoot := filepath.Join(req.Home, ".gemini", "config")
	_, links, messages, skillErrs := installer.deps.PlanHostSkillLinks(
		req.Root,
		filepath.Join(geminiRoot, "skills"),
		req.SkillNames,
		"agy",
		req.DryRun,
	)
	plan.Messages(messages)
	plan.Links(links)
	plan.Errs(skillErrs)

	plan.File(installer.writeAgyUserMCP(filepath.Join(geminiRoot, "mcp_config.json"), req))
	plan.File(installer.writeAgyProjectMCP(
		filepath.Join(req.Root, "configs", "agy", "mcp_config.json"),
		"agy_project_mcp_template",
		"./bin/issueops",
		req.DryRun,
	))

	if req.ProjectLocal {
		plan.File(installer.writeAgyProjectMCP(
			filepath.Join(req.Root, ".agents", "mcp_config.json"),
			"agy_project_mcp_config",
			"./bin/"+osfile.ExecutableName("issueops"),
			req.DryRun,
		))
	}

	if req.DryRun {
		plan.Message("dry-run: planned agy native skills and MCP config without writing")
	}
	return plan.Finish()
}
