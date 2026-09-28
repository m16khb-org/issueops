package issueopsbasesync

import (
	"reflect"
	"testing"
)

func TestMissingAuthorityKeepsActiveHolderAndReleasedCompletionRules(t *testing.T) {
	for _, tt := range []struct {
		name  string
		facts AuthorityFacts
		want  []string
	}{
		{name: "active preview", facts: AuthorityFacts{Mode: "preview", LeaseStatus: "active"}},
		{name: "active different holder", facts: AuthorityFacts{Mode: "apply", LeaseStatus: "active"}, want: []string{"lease_holder"}},
		{name: "active holder", facts: AuthorityFacts{Mode: "apply", LeaseStatus: "active", HolderMatches: true}},
		{name: "claimable", facts: AuthorityFacts{Mode: "apply", LeaseStatus: "claimable"}, want: []string{"released_completion_authority"}},
		{name: "released without completion", facts: AuthorityFacts{Mode: "apply", LeaseStatus: "released"}, want: []string{"released_completion_authority"}},
		{name: "legacy completion", facts: AuthorityFacts{Mode: "apply", LeaseStatus: "released", CompletionPresent: true}},
		{name: "missing generation", facts: AuthorityFacts{Mode: "apply", LeaseStatus: "released", CompletionPresent: true, CompletionGeneration: 3}, want: []string{"completion_generation_present"}},
		{name: "stale generation", facts: AuthorityFacts{Mode: "apply", LeaseStatus: "released", CompletionPresent: true, CompletionGeneration: 3, RequestedGeneration: 2}, want: []string{"completion_generation_current"}},
		{name: "released apply", facts: AuthorityFacts{Mode: "apply", LeaseStatus: "released", CompletionPresent: true, CompletionGeneration: 3, RequestedGeneration: 3}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := MissingAuthority(tt.facts); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("missing=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestMissingAuthorityFencesConflictResolution(t *testing.T) {
	base := AuthorityFacts{LeaseStatus: "released", LeaseGeneration: 6, CompletionPresent: true, CompletionGeneration: 5, RequestedGeneration: 5}
	for _, tt := range []struct {
		name, mode string
		resolution AuthorityResolution
		want       []string
	}{
		{name: "apply with existing resolution", mode: "apply", resolution: AuthorityResolution{Present: true}, want: []string{"sync_base_resolution_absent"}},
		{name: "finalize without resolution", mode: "finalize", want: []string{"sync_base_resolution_present"}},
		{name: "abort stale resolution", mode: "abort", resolution: AuthorityResolution{Present: true, LeaseGeneration: 5, CompletionGeneration: 5}, want: []string{"sync_base_resolution_current"}},
		{name: "finalize wrong actor", mode: "finalize", resolution: AuthorityResolution{Present: true, LeaseGeneration: 6, CompletionGeneration: 5}, want: []string{"sync_base_resolution_actor"}},
		{name: "abort same actor", mode: "abort", resolution: AuthorityResolution{Present: true, LeaseGeneration: 6, CompletionGeneration: 5, ActorMatches: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			facts := base
			facts.Mode, facts.Resolution = tt.mode, tt.resolution
			if got := MissingAuthority(facts); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("missing=%q want=%q", got, tt.want)
			}
		})
	}
}
