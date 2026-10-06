package benchmark

import (
	benchmark "issueops/internal/contract/issueopsbenchmark"
	issueopsroutingdomain "issueops/internal/domain/issueopsrouting"
)

// issueOpsSkillRoutingFidelityComplete reports whether the artifact's recorded
// RoutingTrace contains every skill-at-phase pairing the fixture expects (A5).
//
// This is a RECORDED-TRACE PROXY, not proof of live in-CI skill routing. The
// deterministic benchmark run is tautological for this dimension because
// FromFixture synthesizes RoutingTrace from the fixture's own ExpectedRouting
// (parallel to the pioneer keyword proxy's "not live-routing proof" caveat at
// issueops_pioneer_checks.go and issueops_benchmark_score.go). Real
// discrimination comes from (a) the tampered-trace boundary test and (b) future
// REAL traces recorded during non-CI issueops runs.
//
// Each expected (phase, skill) must match a SINGLE trace entry on BOTH fields
// (case-insensitive, trimmed) — not two independent any-scans — so a trace
// where the right skill fired at the WRONG phase fails.
//
// Fixtures without ExpectedRouting are handled as N/A by the scorer and never
// reach this check.
func issueOpsSkillRoutingFidelityComplete(fixture benchmark.IssueOpsBenchmarkFixture, artifact benchmark.IssueOpsBenchmarkArtifact) bool {
	return issueopsroutingdomain.Score(fixture.ExpectedRouting, artifact.RoutingTrace).OK
}
