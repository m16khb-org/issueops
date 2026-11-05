// Package issueopsbasesync owns the pure base synchronization authority rules.
package issueopsbasesync

// AuthorityResolution is the recorded sync-base conflict resolution, when one
// exists: the lease and completion generations it was sealed under and whether
// the current actor is the one that started it.
type AuthorityResolution struct {
	Present              bool
	LeaseGeneration      uint64
	CompletionGeneration uint64
	ActorMatches         bool
}

// AuthorityFacts are the observed lease, completion, and resolution facts that
// MissingAuthority judges for one sync-base Mode (preview, apply, finalize, or
// abort); RequestedGeneration is the completion generation the caller supplied.
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

// MissingAuthority returns the names of the authority gates the caller fails
// for the requested mode, or nil when it may proceed. An active lease needs a
// matching holder (except in preview); a released lease needs a completion
// whose generation equals the requested one and, for apply, finalize, and
// abort, a resolution record consistent with the mode.
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
	return missingResolutionAuthority(facts)
}

// missingResolutionAuthority는 모드별로 sync-base resolution 기록이 요구 조건을 만족하는지 확인한다.
func missingResolutionAuthority(facts AuthorityFacts) []string {
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
