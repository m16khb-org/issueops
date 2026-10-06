package issueops

import (
	benchmark "issueops/internal/contract/issueopsbenchmark"
)

// RoutingFidelityResult reports missing skill-at-phase pairings.
type RoutingFidelityResult struct {
	OK      bool                     `json:"ok"`
	Missing []benchmark.SkillRouting `json:"missing,omitempty"`
}
