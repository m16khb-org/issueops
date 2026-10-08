// Package agentmodel loads the agent model settings of a repository and
// resolves role models with them.
package agentmodel

import (
	"context"
	"fmt"

	contract "issueops/internal/contract/agentmodel"
	domain "issueops/internal/domain/agentmodel"
)

// Settings are the decoded local and global settings files and their paths.
// LocalPath is empty outside a git repository.
type Settings struct {
	Local      contract.Config
	Global     contract.Config
	LocalPath  string
	GlobalPath string
}

// Service resolves role models. Load reads both settings files; its errors
// name the broken file.
type Service struct {
	Load func(ctx context.Context, repo string) (Settings, error)
}

// injectedRoles are the sub-agent roles injected into owner sessions.
var injectedRoles = []contract.Role{
	contract.RolePlanReview, contract.RoleDiffReview, contract.RoleReviewEscalate,
	contract.RoleResearch, contract.RoleReaderCheck,
}

// Resolve fills in.Local and in.Global from repo's settings and resolves.
// omo has no settings section, so its resolution never reads the files.
func (s Service) Resolve(ctx context.Context, repo string, in domain.ResolveInput) (contract.Resolution, error) {
	if in.Host == "claude" || in.Host == "codex" {
		if s.Load == nil {
			return contract.Resolution{}, fmt.Errorf("agent model settings loader is not configured")
		}
		settings, err := s.Load(ctx, repo)
		if err != nil {
			return contract.Resolution{}, err
		}
		in.Local, in.Global = settings.Local, settings.Global
	}
	return domain.Resolve(in)
}

// RoleAgents resolves every injected role for host. omo gets none.
func (s Service) RoleAgents(ctx context.Context, host, repo string) ([]contract.Resolution, error) {
	if host != "claude" && host != "codex" {
		return nil, nil
	}
	if s.Load == nil {
		return nil, fmt.Errorf("agent model settings loader is not configured")
	}
	settings, err := s.Load(ctx, repo)
	if err != nil {
		return nil, err
	}
	resolutions := make([]contract.Resolution, 0, len(injectedRoles))
	for _, role := range injectedRoles {
		resolution, err := domain.Resolve(domain.ResolveInput{Host: host, Role: role, Local: settings.Local, Global: settings.Global})
		if err != nil {
			return nil, err
		}
		resolutions = append(resolutions, resolution)
	}
	return resolutions, nil
}
