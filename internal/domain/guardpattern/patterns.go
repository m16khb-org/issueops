// Package guardpattern holds the lazily compiled regular expressions and marker strings shared by the
// guard checks that scan test and source text for low-signal tests, unstable or networked tests, new symbols,
// and non-determinism in reusable context prefixes.
package guardpattern

import (
	"regexp"
	"sync"
)

// AmbiguousTestName returns the pattern matching vague test names such as TestWorks, TestBasic, TestTest1, or it("works").
var AmbiguousTestName = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b(Test(Works|Basic|Test[0-9]*)|it\s*\(\s*["'](works|basic|test)[^"']*["']|test\s*\(\s*["'](works|basic|test)[^"']*["'])`)
})

// SleepInTest returns the pattern matching sleep and timer calls (time.Sleep, Thread.sleep, sleep(, setTimeout() that make tests slow or flaky.
var SleepInTest = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b(time\.Sleep|Thread\.sleep|sleep\s*\(|setTimeout\s*\()`)
})

// ExternalURL returns the pattern matching http(s) URLs, used to spot tests that depend on external services.
var ExternalURL = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`https?://[^\s"')]+`) })

// SnapshotAssertion returns the pattern matching snapshot and golden-file assertions.
var SnapshotAssertion = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`(?i)(toMatchSnapshot|assert.*golden|golden mismatch)`)
})

// NewSymbol returns the pattern matching a declaration line (func, function, def, class, type, interface, const, let, var); capture group 1 is the declared name.
var NewSymbol = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`^\s*(?:func\s+|function\s+|def\s+|class\s+|type\s+|interface\s+|const\s+|let\s+|var\s+)([A-Za-z_][A-Za-z0-9_]*)`)
})

// ImmutablePrefixMarker is the immutable-prefix determinism guard marker. A file that opts in
// with it declares that it builds context an agent reuses as
// a stable cache prefix; introducing wall-clock/random/nonce values there
// breaks byte-determinism. A line may carry VolatileOKMarker to declare the
// value intentionally lives in a volatile region.
const ImmutablePrefixMarker = "harness:immutable-prefix"

// VolatileOKMarker is the per-line marker declaring that a non-deterministic value intentionally lives in a volatile region.
const VolatileOKMarker = "volatile-ok"

// ContextNonDeterminism returns the pattern matching wall-clock, random, and UUID calls that break byte-determinism of an immutable prefix.
var ContextNonDeterminism = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`(time\.Now\(|time\.Since\(|rand\.(?:Int|Intn|Float32|Float64|Read|Perm|Shuffle)|uuid\.New)`)
})
