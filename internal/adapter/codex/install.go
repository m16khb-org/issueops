// Package codex is the Codex host adapter: it plans and applies the Codex
// user-level install (skill links, ~/.codex config.toml MCP block, lifecycle
// hooks) and reads the result back as native activation evidence. It lives in
// the adapter layer and receives its filesystem and hook helpers through
// Dependencies.
package codex

import (
	"path/filepath"

	"issueops/internal/port"
)

// Installer is the Codex implementation of port.HostInstaller. It holds only
// the injected Dependencies and no state of its own.
type Installer struct{ deps Dependencies }

// NewInstaller returns an Installer that performs its filesystem and hook work through deps.
func NewInstaller(deps Dependencies) Installer { return Installer{deps: deps} }

var _ port.HostInstaller = Installer{}

// Name returns the host name "codex" that selects this installer.
func (Installer) Name() string { return "codex" }

// Install plans (and, unless req.DryRun, writes) the Codex user skill links,
// the global config.toml MCP block, the lifecycle hooks, and the repository
// templates under configs/codex. Per-step errors are collected into the
// returned HostInstallResult rather than aborting at the first failure.
func (installer Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	plan := installer.deps.NewInstallPlan("codex", req.DryRun)

	_, links, messages, skillErrs := installer.deps.PlanHostSkillLinks(req.Root, filepath.Join(req.CodexHome, "skills"), req.SkillNames, "codex", req.DryRun)
	plan.Messages(messages)
	plan.Links(links)
	plan.Errs(skillErrs)

	plan.File(installer.writeGlobalConfig(filepath.Join(req.CodexHome, "config.toml"), req))

	mcpTemplatePath := filepath.Join(req.Root, "configs", "codex", "mcp.config.toml")
	plan.File(installer.deps.WriteTextPlan(mcpTemplatePath, "codex_mcp_template", codexTemplate(req), 0o644, req.DryRun))

	hooksFile, hookMessages, hooksErr := installer.writeCodexHooks(filepath.Join(req.CodexHome, "hooks.json"), req)
	plan.File(hooksFile, hooksErr)
	plan.Messages(hookMessages)

	hooksTemplatePath := filepath.Join(req.Root, "configs", "codex", "hooks.json")
	plan.File(installer.deps.WriteJSONPlan(hooksTemplatePath, "codex_hooks_template", hooksConfigFor("./bin/issueops", ""), 0o644, req.DryRun))

	if req.DryRun {
		plan.Message("dry-run: planned Codex user skill links, MCP config, and lifecycle hooks without writing")
	}

	return plan.Finish()
}
