// Package extensionhost installs the extension-style hosts (Omo, omp) that
// share one config layout and differ only by the Spec values.
package extensionhost

import (
	"path/filepath"

	"issueops/internal/port"
)

// Spec is the per-host difference between extension-style hosts.
type Spec struct {
	// Host is the lowercase host id used in results, plan kinds, and paths.
	Host string
	// DisplayName is the host name used in human-readable messages.
	DisplayName string
	// ConfigRoot is the user config root relative to the home directory.
	ConfigRoot string
	// SkillsRoot is the user skill link root relative to the home directory.
	SkillsRoot string
	// StdioType, when set, is written as the "type" of stdio MCP entries.
	StdioType string
}

// Omo is the Omo host spec.
var Omo = Spec{Host: "omo", DisplayName: "Omo", ConfigRoot: ".omo", SkillsRoot: filepath.Join(".omo", "agent", "skills")}

// Omp is the omp host spec (default-profile agent directory).
var Omp = Spec{Host: "omp", DisplayName: "omp", ConfigRoot: filepath.Join(".omp", "agent"), SkillsRoot: filepath.Join(".omp", "agent", "skills"), StdioType: "stdio"}

type Installer struct {
	spec               Spec
	deps               Dependencies
	lifecycleExtension func(string) string
}

func NewInstaller(spec Spec, deps Dependencies, lifecycleExtension func(string) string) Installer {
	return Installer{spec: spec, deps: deps, lifecycleExtension: lifecycleExtension}
}

func (installer Installer) Name() string { return installer.spec.Host }

func (installer Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	host := installer.spec.Host
	name := installer.spec.DisplayName
	plan := installer.deps.NewInstallPlan(host, req.DryRun)
	_, links, messages, skillErrs := installer.deps.PlanHostSkillLinks(
		req.Root,
		filepath.Join(req.Home, installer.spec.SkillsRoot),
		req.SkillNames,
		host,
		req.DryRun,
	)
	plan.Messages(messages)
	plan.Links(links)
	plan.Errs(skillErrs)

	configRoot := installer.configRoot(req.Home)
	plan.File(installer.writeUserMCP(filepath.Join(configRoot, "mcp.json"), req))
	plan.File(installer.deps.WriteTextPlan(
		filepath.Join(configRoot, "extensions", "issueops.js"),
		host+"_user_lifecycle_extension",
		installer.lifecycleExtension(req.BinPath),
		0o644,
		req.DryRun,
	))
	plan.File(installer.writeProjectMCP(
		filepath.Join(req.Root, "configs", host, "mcp.json"),
		host+"_project_mcp_template",
		req.DryRun,
	))
	plan.File(installer.deps.WriteTextPlan(
		filepath.Join(req.Root, "configs", host, "issueops.js"),
		host+"_lifecycle_extension_template",
		installer.lifecycleExtension("./bin/issueops"),
		0o644,
		req.DryRun,
	))

	if req.ProjectLocal && req.MCPTransport == mcpTransportHTTP {
		plan.File(installer.removeProjectMCP(filepath.Join(req.Root, "."+host, "mcp.json"), req.DryRun))
		plan.Message("project-local " + name + " MCP uses the user-scope issueops HTTP entry; no project entry or secret is written")
	} else if req.ProjectLocal {
		plan.File(installer.writeProjectMCP(
			filepath.Join(req.Root, "."+host, "mcp.json"),
			host+"_project_mcp_config",
			req.DryRun,
		))
	}

	if req.DryRun {
		plan.Message("dry-run: planned " + name + " native skills, MCP config, and lifecycle extension without writing")
	}
	return plan.Finish()
}

func (installer Installer) configRoot(home string) string {
	return filepath.Join(home, installer.spec.ConfigRoot)
}
