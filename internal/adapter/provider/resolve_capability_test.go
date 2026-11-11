package provider

import (
	"strings"
	"testing"

	"issueops/internal/port"
)

// bareProvider implements only the mandatory port.IssueProvider surface; the
// embedded nil interface is never called.
type bareProvider struct {
	port.IssueProvider
}

func (bareProvider) Name() string { return "fake" }

func TestCapabilityResolversExposeEveryProviderCapability(t *testing.T) {
	wrappers := []struct {
		name    string
		resolve func(string) (bool, error)
	}{
		{"ResolveMilestoneSetter", func(n string) (bool, error) { v, err := ResolveMilestoneSetter(n); return v != nil, err }},
		{"ResolveAssigner", func(n string) (bool, error) { v, err := ResolveAssigner(n); return v != nil, err }},
		{"ResolveRetitler", func(n string) (bool, error) { v, err := ResolveRetitler(n); return v != nil, err }},
		{"ResolveLabeler", func(n string) (bool, error) { v, err := ResolveLabeler(n); return v != nil, err }},
		{"ResolveCommenter", func(n string) (bool, error) { v, err := ResolveCommenter(n); return v != nil, err }},
		{"ResolvePullRequestMerger", func(n string) (bool, error) { v, err := ResolvePullRequestMerger(n); return v != nil, err }},
	}
	for _, provider := range []string{"github", "gitlab"} {
		for _, w := range wrappers {
			t.Run(provider+"/"+w.name, func(t *testing.T) {
				present, err := w.resolve(provider)
				if err != nil {
					t.Fatalf("%s(%q) error: %v", w.name, provider, err)
				}
				if !present {
					t.Fatalf("%s(%q) returned a nil capability", w.name, provider)
				}
			})
		}
	}
}

func TestCapabilityRefusesProviderWithoutTheCapability(t *testing.T) {
	got, err := capability[port.IssueProviderMilestoneSetter]("fake", bareProvider{}, "set milestones")
	if err == nil {
		t.Fatal("capability should refuse a provider without the capability")
	}
	if want := `provider "fake" cannot set milestones`; err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
	if got != nil {
		t.Fatalf("capability returned %v, want the zero value", got)
	}
}

func TestCapabilityResolversPropagateUnknownProvider(t *testing.T) {
	resolvers := map[string]func(string) error{
		"ResolveMilestoneSetter":   func(n string) error { _, err := ResolveMilestoneSetter(n); return err },
		"ResolveAssigner":          func(n string) error { _, err := ResolveAssigner(n); return err },
		"ResolveRetitler":          func(n string) error { _, err := ResolveRetitler(n); return err },
		"ResolveLabeler":           func(n string) error { _, err := ResolveLabeler(n); return err },
		"ResolveCommenter":         func(n string) error { _, err := ResolveCommenter(n); return err },
		"ResolvePullRequestMerger": func(n string) error { _, err := ResolvePullRequestMerger(n); return err },
	}
	for name, resolve := range resolvers {
		t.Run(name, func(t *testing.T) {
			err := resolve("bad")
			if err == nil || !strings.Contains(err.Error(), "unknown provider") {
				t.Fatalf("%s(%q) error = %v, want unknown provider", name, "bad", err)
			}
		})
	}
}
