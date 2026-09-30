// Package issueopsbasesync owns the pure base synchronization authority rules.
package issueopsbasesync

type AuthorityResolution struct {
	Present              bool
	LeaseGeneration      uint64
	CompletionGeneration uint64
	ActorMatches         bool
}

type AuthorityFacts struct {
	Mode                 string
	LeaseStatus          string
	LeaseGeneration      uint64
	HolderMatches        bool
	CompletionPresent    bool
	CompletionGeneration uint64
	RequestedGeneration  uint64
	Resolution           AuthorityResolution
}

func MissingAuthority(facts AuthorityFacts) []string {
	if facts.LeaseStatus == "active" {
		if facts.Mode != "preview" && !facts.HolderMatches {
			return []string{"lease_holder"}
		}
		return nil
	}
	if facts.LeaseStatus != "released" || !facts.CompletionPresent {
		return []string{"released_completion_authority"}
	}
	if facts.CompletionGeneration == 0 {
		return nil
	}
	if facts.RequestedGeneration == 0 {
		return []string{"completion_generation_present"}
	}
	if facts.RequestedGeneration != facts.CompletionGeneration {
		return []string{"completion_generation_current"}
	}
	switch facts.Mode {
	case "apply":
		if facts.Resolution.Present {
			return []string{"sync_base_resolution_absent"}
		}
	case "finalize", "abort":
		if !facts.Resolution.Present {
			return []string{"sync_base_resolution_present"}
		}
		if facts.Resolution.LeaseGeneration != facts.LeaseGeneration || facts.Resolution.CompletionGeneration != facts.CompletionGeneration {
			return []string{"sync_base_resolution_current"}
		}
		if !facts.Resolution.ActorMatches {
			return []string{"sync_base_resolution_actor"}
		}
	}
	return nil
}
