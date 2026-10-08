package agentmodel

import (
	"strings"
	"testing"

	contract "issueops/internal/contract/agentmodel"
)

func TestResolve(t *testing.T) {
	local := contract.Config{Version: 1, Claude: map[contract.Role]contract.Layer{
		contract.RoleImplement:  {Effort: "xhigh"},
		contract.RoleDiffReview: {Effort: "max"},
	}, Codex: map[contract.Role]contract.Layer{
		contract.RoleResearch: {Effort: "low"},
	}}
	global := contract.Config{Version: 1, Claude: map[contract.Role]contract.Layer{
		contract.RoleImplement: {Model: "claude-sonnet-5-5", Effort: "low"},
	}, Codex: map[contract.Role]contract.Layer{
		contract.RoleResearch:       {Model: "gpt-6-luna-mini"},
		contract.RoleChildImplement: {Effort: "medium"},
	}}
	tests := []struct {
		name string
		in   ResolveInput
		want contract.Resolution
	}{
		{"builtin", ResolveInput{Host: "claude", Role: contract.RoleImplement},
			res("claude", contract.RoleImplement, "claude-opus-5-5", "high", "default", "default")},
		{"field merge across local and global", ResolveInput{Host: "codex", Role: contract.RoleResearch, Local: local, Global: global},
			res("codex", contract.RoleResearch, "gpt-6-luna-mini", "low", "global", "local")},
		{"local beats global", ResolveInput{Host: "claude", Role: contract.RoleImplement, Local: local, Global: global},
			res("claude", contract.RoleImplement, "claude-sonnet-5-5", "xhigh", "global", "local")},
		{"flag beats local", ResolveInput{Host: "claude", Role: contract.RoleImplement, Local: local, Global: global, Flag: contract.Layer{Model: "opus"}},
			res("claude", contract.RoleImplement, "opus", "xhigh", "flag", "local")},
		{"child inherits implement", ResolveInput{Host: "claude", Role: contract.RoleChildImplement, Local: local, Global: global},
			res("claude", contract.RoleChildImplement, "claude-sonnet-5-5", "xhigh", "inherited", "inherited")},
		{"child setting wins over inheritance", ResolveInput{Host: "codex", Role: contract.RoleChildImplement, Global: global},
			res("codex", contract.RoleChildImplement, "gpt-6.1-sol", "medium", "inherited", "global")},
		{"docs-only lowers default effort", ResolveInput{Host: "codex", Role: contract.RoleDiffReview, Tier: "docs-only"},
			res("codex", contract.RoleDiffReview, "gpt-6-astra", "medium", "default", "default")},
		{"docs-only keeps configured effort", ResolveInput{Host: "claude", Role: contract.RoleDiffReview, Tier: "docs-only", Local: local},
			res("claude", contract.RoleDiffReview, "claude-opus-5-5", "max", "default", "local")},
		{"other tiers keep default effort", ResolveInput{Host: "codex", Role: contract.RolePlanReview, Tier: "contract"},
			res("codex", contract.RolePlanReview, "gpt-6-astra", "high", "default", "default")},
		{"round 2 does not escalate", ResolveInput{Host: "claude", Role: contract.RoleDiffReview, Round: 2},
			res("claude", contract.RoleDiffReview, "claude-opus-5-5", "high", "default", "default")},
		{"round 3 steps effort once", ResolveInput{Host: "claude", Role: contract.RoleDiffReview, Round: 3},
			res("claude", contract.RoleDiffReview, "claude-opus-5-5", "xhigh", "inherited", "inherited")},
		{"round 4 steps effort once", ResolveInput{Host: "codex", Role: contract.RolePlanReview, Round: 4},
			res("codex", contract.RolePlanReview, "gpt-6-astra", "xhigh", "inherited", "inherited")},
		{"round 5 caps at max", ResolveInput{Host: "claude", Role: contract.RoleDiffReview, Round: 5, Local: local},
			res("claude", contract.RoleDiffReview, "claude-opus-5-5", "max", "inherited", "inherited")},
		{"round 3 steps from docs-only effort", ResolveInput{Host: "codex", Role: contract.RoleDiffReview, Tier: "docs-only", Round: 3},
			res("codex", contract.RoleDiffReview, "gpt-6-astra", "high", "inherited", "inherited")},
		{"round 3 uses review-escalate settings", ResolveInput{Host: "claude", Role: contract.RoleDiffReview, Round: 3,
			Global: contract.Config{Version: 1, Claude: map[contract.Role]contract.Layer{contract.RoleReviewEscalate: {Model: "claude-opus-5-1"}}}},
			res("claude", contract.RoleDiffReview, "claude-opus-5-1", "xhigh", "global", "inherited")},
		{"review-escalate equals diff-review round 3", ResolveInput{Host: "codex", Role: contract.RoleReviewEscalate},
			res("codex", contract.RoleReviewEscalate, "gpt-6-astra", "xhigh", "inherited", "inherited")},
		{"omo ignores settings", ResolveInput{Host: "omo", Role: contract.RoleImplement, Local: local, Global: global},
			res("omo", contract.RoleImplement, "chatgpt-subscription/gpt-6-sol", "max", "default", "default")},
		{"omo has no reader-check default", ResolveInput{Host: "omo", Role: contract.RoleReaderCheck},
			res("omo", contract.RoleReaderCheck, "", "", "", "")},
		{"claude reader-check", ResolveInput{Host: "claude", Role: contract.RoleReaderCheck},
			res("claude", contract.RoleReaderCheck, "claude-haiku-5-5", "medium", "default", "default")},
		{"codex reader-check", ResolveInput{Host: "codex", Role: contract.RoleReaderCheck},
			res("codex", contract.RoleReaderCheck, "gpt-6-luna", "low", "default", "default")},
		{"claude research", ResolveInput{Host: "claude", Role: contract.RoleResearch},
			res("claude", contract.RoleResearch, "claude-sonnet-5-5", "medium", "default", "default")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Resolve(test.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("Resolve = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestResolveRejectsUnknownHostRoleAndEffort(t *testing.T) {
	for name, in := range map[string]ResolveInput{
		"host":   {Host: "gemini", Role: contract.RoleImplement},
		"role":   {Host: "claude", Role: "reviewer"},
		"effort": {Host: "claude", Role: contract.RoleImplement, Flag: contract.Layer{Effort: "ultra"}},
	} {
		if _, err := Resolve(in); err == nil {
			t.Fatalf("%s: Resolve accepted %+v", name, in)
		}
	}
}

func TestValidateSetting(t *testing.T) {
	failures := []struct {
		host string
		role contract.Role
		in   contract.Layer
		want string
	}{
		{"omo", contract.RoleImplement, contract.Layer{Model: "x"}, "claude, codex"},
		{"claude", "reviewer", contract.Layer{Model: "x"}, "implement"},
		{"claude", contract.RoleImplement, contract.Layer{Effort: "ultra"}, "low, medium, high, xhigh, max"},
		{"codex", contract.RoleImplement, contract.Layer{Effort: "off"}, "minimal, low"},
		{"claude", contract.RoleImplement, contract.Layer{}, "model or effort"},
		{"claude", contract.RoleImplement, contract.Layer{Model: "--dangerous"}, "model"},
		{"claude", contract.RoleImplement, contract.Layer{Model: "claude opus"}, "model"},
	}
	for _, test := range failures {
		if _, err := ValidateSetting(test.host, test.role, test.in, nil); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("ValidateSetting(%s,%s,%+v) err = %v, want it to mention %q", test.host, test.role, test.in, err, test.want)
		}
	}

	warnings, err := ValidateSetting("claude", contract.RoleResearch, contract.Layer{Model: "gpt-6-luna"}, nil)
	if err != nil || len(warnings) != 1 {
		t.Fatalf("a non-Claude model id must warn, got %v %v", warnings, err)
	}
	for _, model := range []string{"opus", "sonnet", "haiku", "fable", "claude-opus-5-5"} {
		if warnings, err := ValidateSetting("claude", contract.RoleResearch, contract.Layer{Model: model}, nil); err != nil || len(warnings) != 0 {
			t.Fatalf("%s: warnings=%v err=%v", model, warnings, err)
		}
	}
	catalog := []string{"gpt-6-luna", "gpt-6-astra"}
	if warnings, _ := ValidateSetting("codex", contract.RoleResearch, contract.Layer{Model: "gpt-7"}, catalog); len(warnings) != 1 {
		t.Fatalf("a model missing from the Codex catalog must warn, got %v", warnings)
	}
	if warnings, _ := ValidateSetting("codex", contract.RoleResearch, contract.Layer{Model: "gpt-6-luna"}, catalog); len(warnings) != 0 {
		t.Fatalf("a catalog model must not warn, got %v", warnings)
	}
	if warnings, _ := ValidateSetting("codex", contract.RoleResearch, contract.Layer{Model: "gpt-7"}, nil); len(warnings) != 0 {
		t.Fatalf("an empty catalog must not warn, got %v", warnings)
	}
}

func TestValidateConfig(t *testing.T) {
	if err := ValidateConfig(contract.Config{Version: 2}); err == nil {
		t.Fatal("version 2 must be rejected")
	}
	if err := ValidateConfig(contract.Config{Version: 1, Claude: map[contract.Role]contract.Layer{"reviewer": {Model: "opus"}}}); err == nil {
		t.Fatal("an unknown role must be rejected")
	}
	if err := ValidateConfig(contract.Config{Version: 1, Codex: map[contract.Role]contract.Layer{contract.RoleResearch: {Effort: "ultra"}}}); err == nil {
		t.Fatal("an unsupported effort must be rejected")
	}
	if err := ValidateConfig(contract.Config{}); err != nil {
		t.Fatalf("an absent file decodes to the zero config, which must be valid: %v", err)
	}
}

// Fable is a manual-only choice: no default, inheritance, tier, or round may
// produce it.
func TestBuiltinNeverFable(t *testing.T) {
	for _, host := range []string{"claude", "codex", "omo"} {
		for _, role := range Roles() {
			for _, tier := range []string{"", "default", "docs-only", "contract"} {
				for round := 0; round <= 5; round++ {
					got, err := Resolve(ResolveInput{Host: host, Role: role, Tier: tier, Round: round})
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(strings.ToLower(got.Model), "fable") {
						t.Fatalf("%s %s tier=%s round=%d resolved to %s", host, role, tier, round, got.Model)
					}
				}
			}
		}
	}
}

func TestSupportsEffort(t *testing.T) {
	if !SupportsEffort("claude", "") || !SupportsEffort("omo", "off") || SupportsEffort("claude", "minimal") || SupportsEffort("gemini", "") {
		t.Fatal("effort ladder changed")
	}
}

func res(host string, role contract.Role, model, effort, modelSource, effortSource string) contract.Resolution {
	return contract.Resolution{Host: host, Role: role, Model: model, Effort: effort, ModelSource: modelSource, EffortSource: effortSource}
}
