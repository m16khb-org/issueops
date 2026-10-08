package agentmodel

import (
	"context"
	"errors"
	"testing"

	contract "issueops/internal/contract/agentmodel"
	domain "issueops/internal/domain/agentmodel"
)

func TestResolveUsesRepoSettings(t *testing.T) {
	var loaded string
	service := Service{Load: func(_ context.Context, repo string) (Settings, error) {
		loaded = repo
		return Settings{Local: contract.Config{Version: 1, Codex: map[contract.Role]contract.Layer{contract.RoleResearch: {Effort: "low"}}}}, nil
	}}
	got, err := service.Resolve(context.Background(), "/repo", domain.ResolveInput{Host: "codex", Role: contract.RoleResearch})
	if err != nil || loaded != "/repo" || got.Effort != "low" || got.EffortSource != contract.SourceLocal {
		t.Fatalf("Resolve = %+v, %v (loaded %q)", got, err, loaded)
	}
}

func TestResolveSurfacesBrokenSettingsButNotForOmo(t *testing.T) {
	broken := Service{Load: func(context.Context, string) (Settings, error) {
		return Settings{}, errors.New("/cfg/agent-models.json: unknown field")
	}}
	if _, err := broken.Resolve(context.Background(), "/repo", domain.ResolveInput{Host: "claude", Role: contract.RoleImplement}); err == nil {
		t.Fatal("a broken settings file must not fall back to defaults")
	}
	if got, err := broken.Resolve(context.Background(), "/repo", domain.ResolveInput{Host: "omo", Role: contract.RoleImplement}); err != nil || got.Model == "" {
		t.Fatalf("omo never reads settings: %+v, %v", got, err)
	}
	if _, err := broken.RoleAgents(context.Background(), "codex", "/repo"); err == nil {
		t.Fatal("RoleAgents must surface a broken settings file")
	}
}

func TestRoleAgents(t *testing.T) {
	service := Service{Load: func(context.Context, string) (Settings, error) { return Settings{}, nil }}
	got, err := service.RoleAgents(context.Background(), "claude", "/repo")
	if err != nil || len(got) != len(injectedRoles) {
		t.Fatalf("RoleAgents = %+v, %v", got, err)
	}
	if got[2].Role != contract.RoleReviewEscalate || got[2].Effort != "xhigh" {
		t.Fatalf("review-escalate = %+v", got[2])
	}
	if omo, err := service.RoleAgents(context.Background(), "omo", "/repo"); err != nil || omo != nil {
		t.Fatalf("omo RoleAgents = %+v, %v", omo, err)
	}
}
