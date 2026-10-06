package issueops

import benchmark "issueops/internal/contract/issueopsbenchmark"

type PassPowKPoint = benchmark.PassPowKPoint
type ReliabilityReport = benchmark.ReliabilityReport

// RoutingFidelityResult reports missing skill-at-phase pairings.
type RoutingFidelityResult struct {
	OK      bool           `json:"ok"`
	Missing []SkillRouting `json:"missing,omitempty"`
}
