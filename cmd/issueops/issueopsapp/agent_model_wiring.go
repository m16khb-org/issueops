package issueopsapp

import (
	"context"
	"os"

	"issueops/cmd/issueops/modelcli"
	"issueops/internal/adapter/hostprotocol"
	"issueops/internal/adapter/outbound/agentmodelconfig"
	statestore "issueops/internal/adapter/outbound/state"
	agentmodelapp "issueops/internal/application/agentmodel"
	agentmodelcontract "issueops/internal/contract/agentmodel"
	agentmodeldomain "issueops/internal/domain/agentmodel"
)

func agentModelService() agentmodelapp.Service {
	return agentmodelapp.Service{Load: loadAgentModelSettings}
}

func loadAgentModelSettings(ctx context.Context, repo string) (agentmodelapp.Settings, error) {
	home, _ := os.UserHomeDir()
	files, err := agentmodelconfig.Load(ctx, repo, os.Getenv, home)
	if err != nil {
		return agentmodelapp.Settings{}, err
	}
	return agentmodelapp.Settings{Local: files.Local, Global: files.Global, LocalPath: files.LocalPath, GlobalPath: files.GlobalPath}, nil
}

// resolveAgentModel resolves one role for repo with no flag layer.
func resolveAgentModel(ctx context.Context, host string, role agentmodelcontract.Role, tier, repo string) (agentmodelcontract.Resolution, error) {
	return agentModelService().Resolve(ctx, repo, agentmodeldomain.ResolveInput{Host: host, Role: role, Tier: tier})
}

// roleAgentArgs is the one producer of the role-agent launch arguments that
// Orca, cmux, and `issueops model resolve --agents` inject into a host
// session. Codex role files go under <stateRoot>/agent-roles/.
func roleAgentArgs(ctx context.Context, stateRoot, host, repo string) ([]string, error) {
	resolutions, err := agentModelService().RoleAgents(ctx, host, repo)
	if err != nil {
		return nil, err
	}
	agents := make([]hostprotocol.RoleAgent, 0, len(resolutions))
	for _, resolution := range resolutions {
		agents = append(agents, hostprotocol.RoleAgent{Role: resolution.Role, Model: resolution.Model, Effort: resolution.Effort})
	}
	return hostprotocol.RoleAgentArgs(host, agents, func(content string) (string, error) {
		return agentmodelconfig.WriteRoleFile(stateRoot, content)
	})
}

// preparationAgentModels adapts the agent model settings to the preparation
// service, which resolves owner defaults and Orca role-agent arguments.
type preparationAgentModels struct{ stateRoot string }

func (m preparationAgentModels) OwnerDefaults(ctx context.Context, host string, role agentmodelcontract.Role, repo string) (string, string, error) {
	resolution, err := resolveAgentModel(ctx, host, role, "", repo)
	return resolution.Model, resolution.Effort, err
}

func (m preparationAgentModels) RoleAgentArgs(ctx context.Context, host, repo string) ([]string, error) {
	return roleAgentArgs(ctx, m.stateRoot, host, repo)
}

// stateRoleAgentArgs binds roleAgentArgs to one state root for the resume
// and reconcile services.
func stateRoleAgentArgs(stateRoot string) func(ctx context.Context, host, repo string) ([]string, error) {
	return func(ctx context.Context, host, repo string) ([]string, error) {
		return roleAgentArgs(ctx, stateRoot, host, repo)
	}
}

func runModel(args []string) error {
	return modelcli.Run(modelDependencies(), args)
}

func modelDependencies() modelcli.Dependencies {
	home, _ := os.UserHomeDir()
	return modelcli.Dependencies{
		Models:      agentModelService(),
		WriteGlobal: agentmodelconfig.Write,
		WriteLocal: func(ctx context.Context, repo string, cfg agentmodelcontract.Config) (bool, error) {
			local, err := agentmodelconfig.LocalPath(ctx, repo)
			if err != nil {
				return false, err
			}
			return agentmodelconfig.WriteLocal(local, cfg)
		},
		CodexCatalog: func() []string { return agentmodelconfig.CodexCatalog(os.Getenv, home) },
		RoleAgentArgs: func(ctx context.Context, host, repo string) ([]string, error) {
			return roleAgentArgs(ctx, statestore.StateDir(), host, repo)
		},
		PrintArgv: hostprotocol.BuildPrintArgv,
		Getwd:     os.Getwd,
		Stdout:    os.Stdout,
	}
}

// memoizedReviewModel resolves review roles for one `issueops next` call. It
// loads each repository's settings once: next resolves the review twice
// (before and after the change tier is known), and every load runs git.
func memoizedReviewModel() func(host string, role agentmodelcontract.Role, tier, repo string) (string, string, error) {
	type loaded struct {
		settings agentmodelapp.Settings
		err      error
	}
	cache := map[string]loaded{}
	service := agentmodelapp.Service{Load: func(ctx context.Context, repo string) (agentmodelapp.Settings, error) {
		if hit, ok := cache[repo]; ok {
			return hit.settings, hit.err
		}
		settings, err := loadAgentModelSettings(ctx, repo)
		cache[repo] = loaded{settings, err}
		return settings, err
	}}
	return func(host string, role agentmodelcontract.Role, tier, repo string) (string, string, error) {
		resolution, err := service.Resolve(context.Background(), repo, agentmodeldomain.ResolveInput{Host: host, Role: role, Tier: tier})
		return resolution.Model, resolution.Effort, err
	}
}
