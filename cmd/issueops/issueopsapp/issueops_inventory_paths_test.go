package issueopsapp

import "testing"

type countingPathNormalizer struct{ normalize, canonical map[string]int }

func (paths countingPathNormalizer) Normalize(path string) string {
	paths.normalize[path]++
	return "repo:" + path
}

func (paths countingPathNormalizer) Canonical(path string) string {
	paths.canonical[path]++
	return "canonical:" + path
}

// One request resolves each path once; the next request resolves again, so a
// long-running service never keeps a repository identity past one call.
func TestCallPathNormalizerResolvesEachPathOncePerCall(t *testing.T) {
	base := countingPathNormalizer{normalize: map[string]int{}, canonical: map[string]int{}}
	call := newCallPathNormalizer(base)
	for range 3 {
		if got := call.Normalize("/w"); got != "repo:/w" {
			t.Fatalf("Normalize = %q", got)
		}
		if got := call.Canonical("/w"); got != "canonical:/w" {
			t.Fatalf("Canonical = %q", got)
		}
	}
	call.Normalize("/other")
	if base.normalize["/w"] != 1 || base.canonical["/w"] != 1 || base.normalize["/other"] != 1 {
		t.Fatalf("resolutions within one call = %v %v, want one per path", base.normalize, base.canonical)
	}
	newCallPathNormalizer(base).Normalize("/w")
	if base.normalize["/w"] != 2 {
		t.Fatalf("a new call reused a resolution: %v", base.normalize)
	}
}
