package omp

import (
	"path/filepath"

	"issueops/internal/port"
)

type Installer struct {
	deps               Dependencies
	lifecycleExtension func(string) string
}

func NewInstaller(deps Dependencies, lifecycleExtension func(string) string) Installer {
	return Installer{deps: deps, lifecycleExtension: lifecycleExtension}
}

func (Installer) Name() string { return "omp" }

func (installer Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	plan := installer.deps.NewInstallPlan("omp", req.DryRun)
	agentRoot := ompAgentRoot(req.Home)
	_, links, messages, skillErrs := installer.deps.PlanHostSkillLinks(
		req.Root,
		filepath.Join(agentRoot, "skills"),
		req.SkillNames,
		"omp",
		req.DryRun,
	)
	plan.Messages(messages)
	plan.Links(links)
	plan.Errs(skillErrs)

	plan.File(installer.writeOmpUserMCP(filepath.Join(agentRoot, "mcp.json"), req))
	plan.File(installer.deps.WriteTextPlan(
		filepath.Join(agentRoot, "extensions", "issueops.js"),
		"omp_user_lifecycle_extension",
		installer.lifecycleExtension(req.BinPath),
		0o644,
		req.DryRun,
	))
	plan.File(installer.writeOmpProjectMCP(
		filepath.Join(req.Root, "configs", "omp", "mcp.json"),
		"omp_project_mcp_template",
		req.DryRun,
	))
	plan.File(installer.deps.WriteTextPlan(
		filepath.Join(req.Root, "configs", "omp", "issueops.js"),
		"omp_lifecycle_extension_template",
		installer.lifecycleExtension("./bin/issueops"),
		0o644,
		req.DryRun,
	))

	if req.ProjectLocal && req.MCPTransport == mcpTransportHTTP {
		plan.File(installer.removeOmpProjectMCP(filepath.Join(req.Root, ".omp", "mcp.json"), req.DryRun))
		plan.Message("project-local omp MCP uses the user-scope issueops HTTP entry; no project entry or secret is written")
	} else if req.ProjectLocal {
		plan.File(installer.writeOmpProjectMCP(
			filepath.Join(req.Root, ".omp", "mcp.json"),
			"omp_project_mcp_config",
			req.DryRun,
		))
	}

	if req.DryRun {
		plan.Message("dry-run: planned omp native skills, MCP config, and lifecycle extension without writing")
	}
	return plan.Finish()
}

// ompAgentRoot is the default-profile omp agent directory.
func ompAgentRoot(home string) string {
	return filepath.Join(home, ".omp", "agent")
}
