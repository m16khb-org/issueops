package codex

import (
	"path/filepath"

	"issueops/internal/port"
)

type Installer struct{ deps Dependencies }

func NewInstaller(deps Dependencies) Installer { return Installer{deps: deps} }

func (Installer) Name() string { return "codex" }

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
	plan.File(installer.deps.WriteJSONPlan(hooksTemplatePath, "codex_hooks_template", codexHooksConfig("./bin/issueops"), 0o644, req.DryRun))

	if req.DryRun {
		plan.Message("dry-run: planned Codex user skill links, MCP config, and lifecycle hooks without writing")
	}

	return plan.Finish()
}
