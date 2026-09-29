package hookcatalog

import (
	"os"
	"reflect"
	"testing"

	hookcontract "issueops/internal/contract/hookprompt"
)

func TestCatalogConfigsKeepIndependentReaders(t *testing.T) {
	input, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	previous := os.Stdin
	os.Stdin = input
	t.Cleanup(func() { os.Stdin = previous })
	var calls []string
	var outputs []string
	makeConfig := func(name string) Config {
		reader := func(repo string) hookcontract.ProjectDocCatalogContext {
			calls = append(calls, name+":"+repo)
			return hookcontract.ProjectDocCatalogContext{ShouldInject: true, Compact: name, UserView: name}
		}
		return Config{BuildCatalog: reader,
			ResolveTarget: func(string) string { return name + "-repo" },
			PrintJSON: func(v any) error {
				outputs = append(outputs, v.(hookcontract.ProjectDocCatalogContext).Compact)
				return nil
			},
		}
	}
	a, b := makeConfig("a"), makeConfig("b")
	for _, c := range []Config{a, b, a} {
		if err := RunPostCompact([]string{"--json"}, c); err != nil {
			t.Fatal(err)
		}
		if err := RunSessionStart([]string{"--json"}, c); err != nil {
			t.Fatal(err)
		}
	}
	if want := []string{"a:a-repo", "a:a-repo", "b:b-repo", "b:b-repo", "a:a-repo", "a:a-repo"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("reader calls = %v, want %v", calls, want)
	}
	if want := []string{"a", "a", "b", "b", "a", "a"}; !reflect.DeepEqual(outputs, want) {
		t.Fatalf("output = %v, want %v", outputs, want)
	}
}
