// Package traceclassification is the domain-layer rule set that turns a failing trace step
// or failure class into a proposed harness knob, an overfit-risk note, and the
// default verification command. It is pure string classification with no I/O.
package traceclassification

import "strings"

// ProposedKnobForStep returns the smallest harness guardrail to propose for a
// failing step, chosen by keywords in the lower-cased step name (contract,
// redaction, policy, test, build). Unknown steps get a generic recommendation.
func ProposedKnobForStep(step string) string {
	step = strings.ToLower(step)
	switch {
	case strings.Contains(step, "contract") || strings.Contains(step, "golden"):
		return "tighten CLI/MCP contract golden coverage and update schema intentionally"
	case strings.Contains(step, "redaction") || strings.Contains(step, "secret"):
		return "extend redaction audit fixtures before adding any new logging surface"
	case strings.Contains(step, "policy") || strings.Contains(step, "guard"):
		return "add deterministic policy or guard fixture for the repeated failure"
	case strings.Contains(step, "go test") || strings.Contains(step, "test"):
		return "reduce failing test to a fixture-backed regression before broad harness changes"
	case strings.Contains(step, "build"):
		return "keep build failure fix in core/CLI code path and verify release build"
	default:
		return "classify the repeated trace pattern, then add the smallest harness guardrail with a fixture"
	}
}

// OverfitRiskForClass returns a risk note describing how likely tuning the harness
// for a failure class is to overfit. Single-failure and intermittent classes are
// rated high; deterministic and unknown classes are medium.
func OverfitRiskForClass(class string) string {
	class = strings.ToLower(class)
	switch class {
	case "deterministic":
		return "medium: deterministic failures justify a guardrail, but keep it fixture-backed"
	case "intermittent":
		return "high: rerun and isolate flake signals before changing harness prompts or hooks"
	case "single_failure_observation":
		return "high: one fail-fast observation is insufficient for broad harness tuning"
	default:
		return "medium: verify on a synthetic trace before changing shared harness behavior"
	}
}

// DefaultVerificationCommand returns the shell command that re-verifies a failing
// step, chosen by keywords in the lower-cased step name. It falls back to the full
// test suite when no keyword matches.
func DefaultVerificationCommand(step string) string {
	step = strings.ToLower(step)
	switch {
	case strings.Contains(step, "contract") || strings.Contains(step, "golden"):
		return "go test ./cmd/issueops/contractgolden -run Golden -count=1"
	case strings.Contains(step, "policy") || strings.Contains(step, "guard"):
		return "go test ./internal/domain/policy ./internal/domain/guard ./internal/application/policy ./internal/application/guard -count=1"
	case strings.Contains(step, "build"):
		return "go build -o bin/issueops ./cmd/issueops"
	default:
		return "go test ./... -count=1"
	}
}
