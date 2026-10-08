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

// configurableHosts are the hosts a settings file may configure. omo and omp
// keep their built-in defaults.
var configurableHosts = []string{"claude", "codex"}

// Effort ladders, lowest first. The empty effort means "do not pass one".
var effortLadders = map[string][]string{
	"claude": {"low", "medium", "high", "xhigh", "max"},
	"codex":  {"minimal", "low", "medium", "high", "xhigh", "max"},
	"omo":    {"off", "minimal", "low", "medium", "high", "xhigh", "max"},
	"omp":    {"off", "minimal", "low", "medium", "high", "xhigh", "max"},
}

// KnownHost reports whether host has an effort ladder and built-in defaults.
func KnownHost(host string) bool { return effortLadders[host] != nil }

// SupportsEffort reports whether host accepts effort. The empty effort is
// accepted for every known host.
func SupportsEffort(host, effort string) bool {
	ladder := effortLadders[host]
	return ladder != nil && (effort == "" || slices.Contains(ladder, effort))
}

// stepEffort raises effort one rung and stops at the top of the ladder.
func stepEffort(host, effort string) string {
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
}

// builtinLayer returns the built-in layer of a role that has its own default.
func builtinLayer(host string, role contract.Role) (contract.Layer, bool) {
	layer, ok := builtins[host][role]
	return layer, ok
}
