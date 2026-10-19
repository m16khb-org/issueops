// Package opencode installs the opencode v2 host: user skills, the issueops
// entry under mcp.servers in opencode.json, and the lifecycle plugin.
package opencode

import (
	"path/filepath"

	"issueops/internal/port"
)

const host = "opencode"

type Installer struct {
	deps       Dependencies
	lifecycle  func(binPath string) string
	configRoot func(home string) string
}

// NewInstaller takes the plugin generator and the config root resolver from
// the composition root, which also hands both to the native verifiers.
func NewInstaller(deps Dependencies, lifecycle func(string) string, configRoot func(string) string) Installer {
	return Installer{deps: deps, lifecycle: lifecycle, configRoot: configRoot}
}

func (Installer) Name() string { return host }

func (installer Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	plan := installer.deps.NewInstallPlan(host, req.DryRun)
	configRoot := installer.configRoot(req.Home)
	_, links, messages, skillErrs := installer.deps.PlanHostSkillLinks(
		req.Root,
		filepath.Join(configRoot, "skills"),
		req.SkillNames,
		host,
		req.DryRun,
	)
	plan.Messages(messages)
	plan.Links(links)
	plan.Errs(skillErrs)

	warning, err := checkJSONCShadow(filepath.Join(configRoot, "opencode.jsonc"))
	if err != nil {
		plan.Err(err)
	} else if warning != "" {
		plan.Message(warning)
	}
	plan.File(installer.writeUserMCP(filepath.Join(configRoot, "opencode.json"), req))
	plan.File(installer.deps.WriteTextPlan(
		filepath.Join(configRoot, "plugins", "issueops.js"),
		"opencode_user_lifecycle_plugin",
		installer.lifecycle(req.BinPath),
		0o644,
		req.DryRun,
	))
	plan.File(installer.writeProjectMCP(
		filepath.Join(req.Root, "configs", host, "opencode.json"),
		"opencode_project_mcp_template",
		req.DryRun,
	))
	plan.File(installer.deps.WriteTextPlan(
		filepath.Join(req.Root, "configs", host, "issueops.js"),
		"opencode_lifecycle_extension_template",
		installer.lifecycle("./bin/issueops"),
		0o644,
		req.DryRun,
	))

	projectConfig := filepath.Join(req.Root, ".opencode", "opencode.json")
	if req.ProjectLocal && req.MCPTransport == mcpTransportHTTP {
		plan.File(installer.removeProjectMCP(projectConfig, req.DryRun))
		plan.Message("project-local opencode MCP uses the user-scope issueops HTTP entry; no project entry or secret is written")
	} else if req.ProjectLocal {
		plan.File(installer.writeProjectMCP(projectConfig, "opencode_project_mcp_config", req.DryRun))
	}

	if req.DryRun {
		plan.Message("dry-run: planned opencode native skills, MCP config, and lifecycle plugin without writing")
	}
	return plan.Finish()
}
