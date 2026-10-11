package pattern

import (
	"regexp"
	"testing"
)

func TestPatternMatching(t *testing.T) {
	tests := []struct {
		name  string
		re    func() *regexp.Regexp
		input string
		want  bool
	}{
		{"ambiguous go TestWorks", AmbiguousTestName, "func TestWorks(t *testing.T) {", true},
		{"ambiguous go TestBasic", AmbiguousTestName, "func TestBasic(t *testing.T) {", true},
		{"ambiguous go TestTest12", AmbiguousTestName, "func TestTest12(t *testing.T) {", true},
		{"ambiguous go descriptive name", AmbiguousTestName, "func TestParsesHeader(t *testing.T) {", false},
		{"ambiguous js it works", AmbiguousTestName, `it('works fine', () => {})`, true},
		{"ambiguous js test spaced", AmbiguousTestName, `test ( "basic case", () => {})`, true},
		{"ambiguous js descriptive", AmbiguousTestName, `it('rejects empty input', () => {})`, false},

		{"sleep go", SleepInTest, "time.Sleep(time.Second)", true},
		{"sleep java", SleepInTest, "Thread.sleep(100);", true},
		{"sleep js timeout", SleepInTest, "setTimeout(() => {}, 5)", true},
		{"sleep case-insensitive", SleepInTest, "SLEEP (3)", true},
		{"sleep absent", SleepInTest, "asleep := true", false},

		{"snapshot jest", SnapshotAssertion, "expect(x).toMatchSnapshot()", true},
		{"snapshot golden assert", SnapshotAssertion, "assertEqualGolden(t, got)", true},
		{"snapshot plain", SnapshotAssertion, "expect(x).toBe(1)", false},

		{"nondeterminism time.Now", ContextNonDeterminism, "ts := time.Now()", true},
		{"nondeterminism rand.Intn", ContextNonDeterminism, "n := rand.Intn(5)", true},
		{"nondeterminism uuid", ContextNonDeterminism, "id := uuid.New()", true},
		{"nondeterminism rand.Seed not listed", ContextNonDeterminism, "rand.Seed(1)", false},
		{"nondeterminism time.Date", ContextNonDeterminism, "time.Date(2020, 1, 1)", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.re().MatchString(tt.input); got != tt.want {
				t.Fatalf("MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestExternalURLStopsAtDelimiters(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`fetch("https://example.com/a?b=1")`, "https://example.com/a?b=1"},
		{"see http://host.io/x) now", "http://host.io/x"},
		{"url 'https://a.b/c' end", "https://a.b/c"},
		{"url https://a.b/c d", "https://a.b/c"},
	}
	for _, tt := range tests {
		if got := ExternalURL().FindString(tt.input); got != tt.want {
			t.Errorf("FindString(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
	if ExternalURL().MatchString("ftp://example.com") {
		t.Error("ftp scheme must not match")
	}
}

func TestNewSymbolCapturesName(t *testing.T) {
	tests := []struct {
		input string
		want  string // empty means no match
	}{
		{"func Foo() {}", "Foo"},
		{"  type _bar struct{}", "_bar"},
		{"function doIt() {", "doIt"},
		{"def run_all():", "run_all"},
		{"class Widget:", "Widget"},
		{"const Max = 3", "Max"},
		{"let x1 = 2", "x1"},
		{"var v int", "v"},
		{"interface Shape {", "Shape"},
		{"func (r *R) Method() {}", ""},
		{"funcFoo()", ""},
		{"// func Foo", ""},
		{"func 9bad()", ""},
	}
	for _, tt := range tests {
		m := NewSymbol().FindStringSubmatch(tt.input)
		got := ""
		if m != nil {
			got = m[1]
		}
		if got != tt.want {
			t.Errorf("NewSymbol(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
