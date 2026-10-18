// Package agentmodel resolves which model and effort each IssueOps role runs
// with, per native host.
package agentmodel

import (
	"slices"

	contract "issueops/internal/contract/agentmodel"
)

// docsOnlyReviewEffort is the review effort for documentation-only changes
// when the user has not configured one.
const docsOnlyReviewEffort = "medium"

var roles = []contract.Role{
	contract.RoleImplement, contract.RoleChildImplement, contract.RolePlanReview, contract.RoleDiffReview,
	contract.RoleReviewEscalate, contract.RoleResearch, contract.RoleReaderCheck,
}

// Roles returns every role in display order.
func Roles() []contract.Role { return slices.Clone(roles) }

func knownRole(role contract.Role) bool { return slices.Contains(roles, role) }

// configurableHosts are the hosts a settings file may configure.
var configurableHosts = []string{"claude", "codex", "omo", "omp", "opencode"}

// ConfigurableHosts returns the hosts a settings file may configure.
func ConfigurableHosts() []string { return slices.Clone(configurableHosts) }

// Configurable reports whether a settings file may configure host.
func Configurable(host string) bool { return slices.Contains(configurableHosts, host) }

// Effort ladders, lowest first. The empty effort means "do not pass one".
var effortLadders = map[string][]string{
	"claude": {"low", "medium", "high", "xhigh", "max"},
	"codex":  {"minimal", "low", "medium", "high", "xhigh", "max"},
	"omo":    {"off", "minimal", "low", "medium", "high", "xhigh", "max"},
	"omp":    {"off", "minimal", "low", "medium", "high", "xhigh", "max"},
	// opencode names efforts after the model's own variants; this ladder is
	// their union, and an unknown variant fails when opencode runs.
	"opencode": {"none", "minimal", "low", "medium", "high", "xhigh", "max"},
}

// modelDefinedEffortHosts take effort as a model variant, so issueops does not
// derive a new value from it: each model offers a different set, and a derived
// variant the model lacks fails the run.
var modelDefinedEffortHosts = []string{"opencode"}

func modelDefinedEffort(host string) bool { return slices.Contains(modelDefinedEffortHosts, host) }

// KnownHost reports whether host has an effort ladder and built-in defaults.
func KnownHost(host string) bool { return effortLadders[host] != nil }

// SupportsEffort reports whether host accepts effort. The empty effort is
// accepted for every known host.
func SupportsEffort(host, effort string) bool {
	ladder := effortLadders[host]
	return ladder != nil && (effort == "" || slices.Contains(ladder, effort))
}

// stepEffort raises effort one rung and stops at the top of the ladder. A
// model-defined effort is kept as is.
func stepEffort(host, effort string) string {
	if modelDefinedEffort(host) {
		return effort
	}
	ladder := effortLadders[host]
	i := slices.Index(ladder, effort)
	if i < 0 || i == len(ladder)-1 {
		return effort
	}
	return ladder[i+1]
}

// builtins holds the roles with their own defaults. child-implement and
// review-escalate are derived from implement and diff-review. Fable never
// appears here: it is a manual-only choice.
var builtins = map[string]map[contract.Role]contract.Layer{
	"claude": {
		contract.RoleImplement:   {Model: "claude-opus-5-5", Effort: "high"},
		contract.RolePlanReview:  {Model: "claude-opus-5-5", Effort: "high"},
		contract.RoleDiffReview:  {Model: "claude-opus-5-5", Effort: "high"},
		contract.RoleResearch:    {Model: "claude-sonnet-5-5", Effort: "medium"},
		contract.RoleReaderCheck: {Model: "claude-haiku-5-5", Effort: "medium"},
	},
	"codex": {
		contract.RoleImplement:   {Model: "gpt-6.1-sol", Effort: "high"},
		contract.RolePlanReview:  {Model: "gpt-6-astra", Effort: "high"},
		contract.RoleDiffReview:  {Model: "gpt-6-astra", Effort: "high"},
		contract.RoleResearch:    {Model: "gpt-6-luna", Effort: "medium"},
		contract.RoleReaderCheck: {Model: "gpt-6-luna", Effort: "low"},
	},
	"omo": {
		contract.RoleImplement:  {Model: "chatgpt-subscription/gpt-6-sol", Effort: "max"},
		contract.RolePlanReview: {Model: "chatgpt-subscription/gpt-6-astra", Effort: "max"},
		contract.RoleDiffReview: {Model: "chatgpt-subscription/gpt-6-astra", Effort: "max"},
		contract.RoleResearch:   {Model: "chatgpt-subscription/gpt-6-luna", Effort: "medium"},
	},
	"omp": {
		contract.RoleImplement:   {Model: "anthropic/claude-opus-5-5", Effort: "high"},
		contract.RolePlanReview:  {Model: "anthropic/claude-opus-5-5", Effort: "high"},
		contract.RoleDiffReview:  {Model: "anthropic/claude-opus-5-5", Effort: "high"},
		contract.RoleResearch:    {Model: "anthropic/claude-sonnet-5-5", Effort: "medium"},
		contract.RoleReaderCheck: {Model: "anthropic/claude-haiku-5-5", Effort: "medium"},
	},
	// The free opencode Zen model offers the low, high, and max variants.
	"opencode": {
		contract.RoleImplement:   {Model: "opencode/deepseek-v4.1-flash-free", Effort: "max"},
		contract.RolePlanReview:  {Model: "opencode/deepseek-v4.1-flash-free", Effort: "max"},
		contract.RoleDiffReview:  {Model: "opencode/deepseek-v4.1-flash-free", Effort: "max"},
		contract.RoleResearch:    {Model: "opencode/deepseek-v4.1-flash-free", Effort: "high"},
		contract.RoleReaderCheck: {Model: "opencode/deepseek-v4.1-flash-free", Effort: "low"},
	},
}

// builtinLayer returns the built-in layer of a role that has its own default.
func builtinLayer(host string, role contract.Role) (contract.Layer, bool) {
	layer, ok := builtins[host][role]
	return layer, ok
}
