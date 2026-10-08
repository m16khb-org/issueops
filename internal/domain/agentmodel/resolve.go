package agentmodel

import (
	"fmt"
	"strings"

	contract "issueops/internal/contract/agentmodel"
)

// ResolveInput is everything Resolve needs. Local and Global are the decoded
// settings files; a missing file is the zero Config.
type ResolveInput struct {
	Host   string
	Role   contract.Role
	Tier   string
	Round  int
	Flag   contract.Layer
	Local  contract.Config
	Global contract.Config
}

type field struct{ value, source string }

// Resolve applies flag > local > global > built-in per field, then the role
// rules: child-implement inherits implement, docs-only lowers only a
// built-in review effort, and review rounds 3 and later use review-escalate
// or one effort rung above the review.
func Resolve(in ResolveInput) (contract.Resolution, error) {
	host := strings.ToLower(strings.TrimSpace(in.Host))
	if !KnownHost(host) {
		return contract.Resolution{}, fmt.Errorf("unknown host %q (use claude, codex, omo, or omp)", in.Host)
	}
	if !knownRole(in.Role) {
		return contract.Resolution{}, fmt.Errorf("unknown role %q (use %s)", in.Role, joinRoles())
	}
	if err := validateLayer(host, in.Flag); err != nil {
		return contract.Resolution{}, err
	}
	role, round := in.Role, in.Round
	if role == contract.RoleReviewEscalate {
		role, round = contract.RoleDiffReview, max(round, 3)
	}
	model, effort := resolveRole(host, role, in)
	if (role == contract.RolePlanReview || role == contract.RoleDiffReview) && round >= 3 {
		escModel, escEffort := configured(host, contract.RoleReviewEscalate, in)
		if escModel.source == "" {
			escModel = field{model.value, contract.SourceInherited}
		}
		if escEffort.source == "" {
			escEffort = field{stepEffort(host, effort.value), contract.SourceInherited}
		}
		model, effort = escModel, escEffort
	}
	if in.Flag.Model != "" {
		model = field{in.Flag.Model, contract.SourceFlag}
	}
	if in.Flag.Effort != "" {
		effort = field{in.Flag.Effort, contract.SourceFlag}
	}
	return contract.Resolution{
		Host: host, Role: in.Role,
		Model: model.value, Effort: effort.value,
		ModelSource: model.source, EffortSource: effort.source,
	}, nil
}

func resolveRole(host string, role contract.Role, in ResolveInput) (field, field) {
	model, effort := configured(host, role, in)
	if role == contract.RoleChildImplement {
		parentModel, parentEffort := resolveRole(host, contract.RoleImplement, in)
		if model.source == "" {
			model = field{parentModel.value, contract.SourceInherited}
		}
		if effort.source == "" {
			effort = field{parentEffort.value, contract.SourceInherited}
		}
		return model, effort
	}
	builtin, ok := builtinLayer(host, role)
	if model.source == "" && ok {
		model = field{builtin.Model, contract.SourceDefault}
	}
	if effort.source == "" && ok {
		effort = field{builtin.Effort, contract.SourceDefault}
		if strings.TrimSpace(in.Tier) == "docs-only" && (role == contract.RolePlanReview || role == contract.RoleDiffReview) {
			effort.value = docsOnlyReviewEffort
		}
	}
	return model, effort
}

// configured merges the local and global layers of one role. omo and omp have
// no settings file section, so they always fall through to built-ins.
func configured(host string, role contract.Role, in ResolveInput) (model, effort field) {
	for _, layer := range []struct {
		cfg    contract.Config
		source string
	}{{in.Local, contract.SourceLocal}, {in.Global, contract.SourceGlobal}} {
		entry := hostLayers(layer.cfg, host)[role]
		if model.source == "" && entry.Model != "" {
			model = field{entry.Model, layer.source}
		}
		if effort.source == "" && entry.Effort != "" {
			effort = field{entry.Effort, layer.source}
		}
	}
	return model, effort
}

// hostLayers returns the settings section for host, or nil.
func hostLayers(cfg contract.Config, host string) map[contract.Role]contract.Layer {
	switch host {
	case "claude":
		return cfg.Claude
	case "codex":
		return cfg.Codex
	default:
		return nil
	}
}

func joinRoles() string {
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, string(role))
	}
	return strings.Join(names, ", ")
}
