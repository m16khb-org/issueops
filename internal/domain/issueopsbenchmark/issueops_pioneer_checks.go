package issueopsbenchmark

import (
	"strings"

	issueopscontract "issueops/internal/contract/issueopsbenchmark"
)

// pioneerClause is one required evidence clause: structured clauses need a
// labeled line ("label: value"); otherwise any marker anywhere suffices.
type pioneerClause struct {
	structured bool
	markers    []string
}

func structuredClause(markers ...string) pioneerClause {
	return pioneerClause{structured: true, markers: markers}
}

func anyMarkerClause(markers ...string) pioneerClause {
	return pioneerClause{markers: markers}
}

func (c pioneerClause) satisfiedBy(evidence string) bool {
	if c.structured {
		return hasStructuredClause(evidence, c.markers...)
	}
	return containsAnyFold(evidence, c.markers...)
}

var pioneerSkillSignatures = map[string][]pioneerClause{
	"implementation-planning": {
		structuredClause("repo grounding", "grounding"),
		structuredClause("decision-complete", "decision complete"),
		structuredClause("assumptions", "defaults"),
		structuredClause("unresolved question", "open question", "deferred risk"),
		structuredClause("acceptance criteria", "success criteria"),
	},
	"verified-execution": {
		structuredClause("success criteria", "criteria"),
		structuredClause("evidence artifact", "artifact"),
		structuredClause("cleanup receipt", "cleanup"),
		structuredClause("verification mode", "proportionate"),
		structuredClause("skipped checks", "skipped check"),
	},
	"web-research": {
		structuredClause("source fan-out", "fan-out"),
		structuredClause("source index", "sources"),
		structuredClause("claim verification", "verification status"),
		structuredClause("access boundary", "auth", "paywall", "protected", "inaccessible"),
		anyMarkerClause("retrieval timestamp", "retrieved", "retrieval date", "2026-"),
	},
	"algorithm-optimization": {
		structuredClause("hot path", "profile"),
		structuredClause("complexity", "asymptotic", "big-o", "o("),
		structuredClause("scaling", "n=", "measured at"),
		structuredClause("invariant", "correctness"),
		structuredClause("before/after", "before", "benchmark delta", "measurement"),
		anyMarkerClause("after", "improved"),
	},
	"database-design": {
		structuredClause("schema", "row count", "ddl", "cardinality"),
		structuredClause("explain", "query plan"),
		structuredClause("index"),
		anyMarkerClause("write penalty", "write cost", "insert cost"),
		structuredClause("normalization", "normalisation", "anomaly", "bcnf", "3nf"),
	},
	"issueops-debugging": {
		structuredClause("reproduction", "reproduced"),
		structuredClause("failure signature", "signature"),
		structuredClause("root cause", "hypothesis"),
		structuredClause("isolation", "isolated"),
		structuredClause("minimal fix", "fix scope", "fix boundary"),
		structuredClause("verification", "regression proof"),
	},
	"code-quality-metrics": {
		structuredClause("diff inventory", "scope inventory"),
		anyMarkerClause("untracked"),
		structuredClause("snr"),
		anyMarkerClause("before", "baseline"),
		anyMarkerClause("after", "improved"),
		structuredClause("secondary metric", "second metric"),
		anyMarkerClause("entropy", "redundancy", "overhead"),
		structuredClause("heuristic caveat", "approximation caveat"),
		structuredClause("no-input guard", "zero-input guard"),
	},
	"prompt-engineering": {
		structuredClause("input/output contract", "input contract", "output contract"),
		structuredClause("test suite"),
		structuredClause("adversarial"),
		structuredClause("one-variable", "one variable"),
		structuredClause("privacy", "hidden chain-of-thought", "tool truth"),
	},
	"git-operations": {
		structuredClause("git state proof", "state proof"),
		structuredClause("recovery path", "backup ref"),
		structuredClause("destructive confirmation", "confirmation gate"),
		structuredClause("atomic scope", "atomic"),
		structuredClause("force-with-lease", "--force-with-lease"),
	},
	"requirements-analysis": {
		structuredClause("document scope"),
		structuredClause("ocr evidence", "screenshot evidence"),
		structuredClause("requirement ledger"),
		structuredClause("contradiction"),
		structuredClause("risk-driven recommendation"),
	},
	"design-review": {
		structuredClause("essential complexity"),
		structuredClause("accidental complexity"),
		structuredClause("second-system effect"),
		structuredClause("conceptual integrity"),
		structuredClause("go/no-go verdict", "no-go verdict"),
	},
	"meeting-notes": {
		structuredClause("source fidelity"),
		structuredClause("decision log"),
		structuredClause("action owners"),
		structuredClause("uncertainty"),
		structuredClause("canvas handoff"),
	},
	"issueops": {
		structuredClause("durable state record", "state record"),
		structuredClause("phase routing", "routing"),
		structuredClause("flow evidence", "issue", "plan", "tdd"),
		structuredClause("hook boundary", "hook"),
		structuredClause("cleanup/readiness", "readiness evidence", "cleanup"),
	},
}

// PioneerSkillEvidenceComplete reports whether the artifact's
// PioneerSkillEvidence carries the distinctive-method signature of the
// fixture's targeted pioneer skill.
//
// This is a deterministic artifact-structure proxy, not proof that the skill
// ran. Each signature requires method-specific labeled clauses (AND) so hollow
// keyword soup does not pass as method evidence.
//
// Fixtures without a pioneer_skill_target are handled as N/A by the scorer
// and never reach this check.
func PioneerSkillEvidenceComplete(fixture issueopscontract.IssueOpsBenchmarkFixture, artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	clauses, ok := pioneerSkillSignatures[strings.ToLower(strings.TrimSpace(fixture.PioneerSkillTarget))]
	if !ok {
		return false
	}
	for _, clause := range clauses {
		if !clause.satisfiedBy(artifact.PioneerSkillEvidence) {
			return false
		}
	}
	return true
}

func hasStructuredClause(text string, markers ...string) bool {
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, ":") {
			continue
		}
		if containsAnyFold(line, markers...) {
			return true
		}
	}
	return false
}
